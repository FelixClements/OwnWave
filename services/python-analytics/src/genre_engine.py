"""Genre engine v2: decide each song's genre and style (ADR 0008).

Replaces the weighted blend in genre_scoring.py. For every song, in order:

1. A **correction** (song, release or artist scope) always wins.
2. **Strong sources** each name one genre: the genre tag, the Discogs styles,
   the MusicBrainz recording genres, the MusicBrainz release genres. If they
   agree, that is the genre; if they disagree, the song is unsure. A strong
   source whose values name more than one genre names none and only fences.
3. **Weak sources** (broad tags, the artist's genres, Last.fm tags, Discogs
   genres) only build the song's **fence**: the overlap of what each allows
   (within one source its values are alternatives, so they add up). An empty
   fence, or a strong source's genre outside it, makes the song unsure.
4. **Release lean**: with no strong source, a song takes the genre that at
   least 70% of its release's decided songs share, if it is inside the fence.
   Compilations skip this. A classifier at least 80% sure of another genre
   makes the song unsure instead.
5. **Classifier**: k nearest neighbours (cosine, similarity-weighted vote)
   over the audio embeddings, among songs with a trusted label whose genre is
   inside the fence. It decides at 60% of the vote or more; below, the song
   is unsure. Trusted labels are the songs decided by a correction or by
   strong sources (specific genre tags, ID-matched Discogs and MusicBrainz
   genres). Broad tags, Last.fm and untagged songs never teach it.
6. A classifier at 80% or more against a specific genre tag keeps the tag
   but flags the song for review.
7. **Style**: from a strong source inside the decided genre, else the
   Discogs400 model's top style inside that genre when it is confident
   enough, else none.

One genre and at most one style per song; there are no secondary genres.
Unsure songs get no genre and join no genre station.

The engine is pure (decide()); the database layer around it loads the
sources, writes track_genre_decisions (migration 036) and
track_station_genres. Run it in bulk against a
copy of the database with `python src/main.py genre-engine` (read only by
default; see run_report()).
"""

from __future__ import annotations

import logging
import time
from dataclasses import dataclass, field
from typing import Any, Dict, FrozenSet, Iterable, List, Optional, Sequence, Tuple

import numpy as np

import genre_catalog as gc

log = logging.getLogger(__name__)

# --- thresholds (ADR 0008) -----------------------------------------------------

# Release lean: share of the release's decided songs that must share a genre.
RELEASE_LEAN = 0.70
# Release lean needs at least this many decided songs on the release.
RELEASE_MIN_DECIDED = 1
# Classifier: neighbours, share of the vote to decide, share that overrules.
CLASSIFIER_K = 20
CLASSIFIER_DECIDES = 0.60
CLASSIFIER_OVERRULES = 0.80
# Discogs400 style: lowest activation of the top style inside the genre.
STYLE_MIN_ACTIVATION = 0.20
# Weak sources: how many artist genres and Last.fm tags count, and the lowest
# Last.fm weight (0-100) that counts.
MAX_ARTIST_GENRES = 8
MAX_LASTFM_TAGS = 10
LASTFM_MIN_WEIGHT = 10

# --- decision sources, unsure reasons, review reasons ---------------------------
# Stored in track_genre_decisions (migration 036, which checks these values).

CORRECTION = "correction"
STRONG_SOURCES = "strong_sources"
RELEASE = "release"
CLASSIFIER = "classifier"
UNSURE = "unsure"

STRONG_DISAGREE = "strong_sources_disagree"
EMPTY_FENCE = "empty_fence"
OUTSIDE_FENCE = "outside_fence"
RELEASE_VS_CLASSIFIER = "release_vs_classifier"
NO_AUDIO = "no_audio"
CLASSIFIER_UNSURE = "classifier_unsure"
UNSURE_REASONS = (STRONG_DISAGREE, EMPTY_FENCE, OUTSIDE_FENCE, RELEASE_VS_CLASSIFIER, NO_AUDIO, CLASSIFIER_UNSURE)

CLASSIFIER_DISAGREES = "classifier_disagrees"

# Source names, as in the "why" breakdown. Strong ones in the order they give
# a style.
GENRE_TAG = "genre_tag"
DISCOGS = "discogs"
MB_RECORDING = "mb_recording"
MB_RELEASE = "mb_release"
ARTIST = "artist"
LASTFM = "lastfm"
STRONG = (GENRE_TAG, DISCOGS, MB_RECORDING, MB_RELEASE)
WEAK = (ARTIST, LASTFM)

# Correction scopes, most specific first.
SONG_SCOPE, RELEASE_SCOPE, ARTIST_SCOPE = "song", "release", "artist"

# app_state key of the classifier's test score (admin Overview, #140).
TEST_SCORE_KEY = "genre_test_score"


# --- inputs ---------------------------------------------------------------------


@dataclass
class SongInput:
    """Everything the engine knows about one track.

    Outside genres are lists of dicts as stored in outside_genres
    ({"name", "count", "kind"}); artist genres are MusicBrainz artist tags
    ({"name", "count"}). `correction` is a genre or style id, already
    resolved to the most specific scope (`correction_scope`).
    """

    track_id: Any
    song_id: Any = None  # copies of a song share it; defaults to track_id
    best_copy: bool = True
    release_id: Any = None
    compilation: bool = False
    genre_tag: Optional[str] = None
    discogs: Sequence[dict] = ()
    mb_recording: Sequence[dict] = ()
    mb_release: Sequence[dict] = ()
    artist_genres: Sequence[dict] = ()
    lastfm: Sequence[dict] = ()
    correction: Optional[str] = None
    correction_scope: Optional[str] = None
    activations: Optional[Sequence[float]] = None
    embedding: Optional[np.ndarray] = None
    # For reports only.
    artist: Optional[str] = None
    title: Optional[str] = None

    def __post_init__(self):
        if self.song_id is None:
            self.song_id = self.track_id


# --- what one source says ---------------------------------------------------------


@dataclass(frozen=True)
class SourceView:
    """One source's say on a song.

    `answer` is the one genre a strong source names; a source that names no
    single genre (any weak source, or a strong one naming several) fences
    the song to `fence` instead. `styles` are the styles it names, in order.
    """

    answer: Optional[str]
    fence: Optional[FrozenSet[str]]
    styles: Tuple[str, ...] = ()

    def as_dict(self) -> dict:
        out: dict = {}
        if self.answer is not None:
            out["genre"] = self.answer
        if self.fence is not None:
            out["fence"] = sorted(self.fence)
        if self.styles:
            out["styles"] = list(self.styles)
        return out


def read_source(translations: Iterable[Optional[gc.Translation]], strong: bool) -> Optional[SourceView]:
    """Fold a source's translated values into one view, or None if it says nothing."""
    found = [t for t in translations if t is not None]
    if not found:
        return None
    specific: List[str] = []
    styles: List[str] = []
    union: set = set()
    for t in found:
        union |= t.fence
        if t.genre is not None and t.genre not in specific:
            specific.append(t.genre)
        if t.style is not None and t.style not in styles:
            styles.append(t.style)
    if strong and len(specific) == 1:
        return SourceView(specific[0], None, tuple(styles))
    return SourceView(None, frozenset(union), tuple(styles))


def _by_count(values: Sequence[dict]) -> List[dict]:
    return sorted(values, key=lambda v: (-(v.get("count") or 0), v.get("name") or ""))


def source_views(song: SongInput, cat: gc.Catalog) -> Dict[str, SourceView]:
    """Each source's view of the song, keyed by source name."""
    views: Dict[str, Optional[SourceView]] = {}
    if song.genre_tag:
        views[GENRE_TAG] = read_source(cat.genre_tag(song.genre_tag), strong=True)
    if song.discogs:
        parents = [v["name"] for v in song.discogs if v.get("kind") == "genre"]
        found = [cat.discogs_genre(n) for n in parents]
        found += [cat.discogs_style(v["name"], parents) for v in song.discogs if v.get("kind") == "style"]
        views[DISCOGS] = read_source(found, strong=True)
    if song.mb_recording:
        views[MB_RECORDING] = read_source((cat.musicbrainz_genre(v["name"]) for v in _by_count(song.mb_recording)), True)
    if song.mb_release:
        views[MB_RELEASE] = read_source((cat.musicbrainz_genre(v["name"]) for v in _by_count(song.mb_release)), True)
    if song.artist_genres:
        top = _by_count(song.artist_genres)[:MAX_ARTIST_GENRES]
        views[ARTIST] = read_source((cat.musicbrainz_genre(v["name"]) for v in top), strong=False)
    if song.lastfm:
        top = [v for v in _by_count(song.lastfm) if (v.get("count") or 0) >= LASTFM_MIN_WEIGHT][:MAX_LASTFM_TAGS]
        views[LASTFM] = read_source((cat.lastfm_tag(v["name"]) for v in top), strong=False)
    return {k: v for k, v in views.items() if v is not None}


def fence_of(views: Dict[str, SourceView]) -> Optional[FrozenSet[str]]:
    """The overlap of every source that fences; None when none does (any genre)."""
    fence: Optional[FrozenSet[str]] = None
    for view in views.values():
        if view.fence is not None:
            fence = view.fence if fence is None else fence & view.fence
    return fence


def resolve_correction(value: Optional[str], cat: gc.Catalog) -> Optional[Tuple[str, Optional[str]]]:
    """(genre, style) of a stored correction: a genre id or a style id. None if unknown."""
    if not value:
        return None
    if value in cat.genres:
        return value, None
    if value in cat.styles:
        return cat.styles[value].genre, value
    return None


# --- results ----------------------------------------------------------------------


@dataclass
class Decision:
    """One song's genre (None when unsure), style and how they were decided."""

    track_id: Any
    genre: Optional[str]
    style: Optional[str]
    source: str
    confidence: Optional[float]
    unsure_reason: Optional[str] = None
    review_reason: Optional[str] = None
    fence: Optional[List[str]] = None
    why: dict = field(default_factory=dict)

    @property
    def unsure(self) -> bool:
        return self.genre is None


@dataclass
class EngineResult:
    decisions: List[Decision]
    test_score: dict
    seconds: float = 0.0  # deciding, without loading
    loaded_seconds: Optional[float] = None
    songs: Sequence[SongInput] = ()


@dataclass
class _Pending:
    song: SongInput
    views: Dict[str, SourceView]
    fence: Optional[FrozenSet[str]]
    why: dict = field(default_factory=dict)
    decision: Optional[Decision] = None
    # The strong-source genre, kept for the review flag.
    tag_genre: Optional[str] = None


# --- classifier -----------------------------------------------------------------


def _normalise(vectors: np.ndarray) -> np.ndarray:
    norms = np.linalg.norm(vectors, axis=1, keepdims=True)
    norms[norms == 0] = 1.0
    return (vectors / norms).astype(np.float32, copy=False)


def knn_votes(
    queries: np.ndarray,
    query_groups: np.ndarray,
    allowed: np.ndarray,
    labels: np.ndarray,
    label_genres: np.ndarray,
    label_groups: np.ndarray,
    k: int = CLASSIFIER_K,
    chunk: int = 1024,
) -> np.ndarray:
    """Similarity-weighted votes per genre for each query.

    queries (n x d) and labels (m x d) are unit vectors. Each query votes with
    its k most similar labels whose genre it allows (allowed: n x genres),
    leaving out labels in its own group (a label held out from its own vote;
    -1 matches no label), each weighted by its cosine similarity (negative
    similarities do not vote). Returns n x genres.
    """
    n, m = len(queries), len(labels)
    votes = np.zeros((n, allowed.shape[1]), dtype=np.float64)
    if n == 0 or m == 0:
        return votes
    kk = min(k, m)
    for start in range(0, n, chunk):
        stop = min(start + chunk, n)
        sims = queries[start:stop] @ labels.T
        sims[query_groups[start:stop, None] == label_groups[None, :]] = -np.inf
        sims[~allowed[start:stop][:, label_genres]] = -np.inf
        idx = np.argpartition(-sims, kk - 1, axis=1)[:, :kk]
        top = np.take_along_axis(sims, idx, axis=1)
        weights = np.where(np.isfinite(top), np.maximum(top, 0.0), 0.0)
        rows = np.repeat(np.arange(stop - start), kk)
        block = votes[start:stop]
        np.add.at(block, (rows, label_genres[idx].ravel()), weights.ravel())
    return votes


@dataclass(frozen=True)
class Vote:
    genre: Optional[str]
    share: float
    runner_ups: Tuple[Tuple[str, float], ...] = ()

    def as_dict(self) -> dict:
        return {
            "genre": self.genre,
            "share": round(self.share, 3),
            "runner_ups": [[g, round(s, 3)] for g, s in self.runner_ups],
        }


def _vote(row: np.ndarray, genre_ids: Sequence[str]) -> Vote:
    total = float(row.sum())
    if total <= 0:
        return Vote(None, 0.0)
    order = np.argsort(-row, kind="stable")
    shares = [(genre_ids[i], float(row[i]) / total) for i in order[:4] if row[i] > 0]
    return Vote(shares[0][0], shares[0][1], tuple(shares[1:]))


# --- the engine ---------------------------------------------------------------------


def decide(
    songs: Sequence[SongInput],
    cat: gc.Catalog,
    model_labels: Optional[Sequence[str]] = None,
    k: int = CLASSIFIER_K,
) -> EngineResult:
    """Decide the genre and style of every song (see the module docstring)."""
    started = time.monotonic()
    genre_ids = list(cat.genres)
    gindex = {g: i for i, g in enumerate(genre_ids)}
    pending = [_first_pass(s, cat) for s in songs]

    # Trusted labels: corrections and agreeing strong sources, one per song.
    has_audio = [i for i, p in enumerate(pending) if p.song.embedding is not None]
    label_rows = [
        i for i in has_audio
        if pending[i].decision is not None and pending[i].song.best_copy
        and pending[i].decision.source in (CORRECTION, STRONG_SOURCES)
    ]
    # Labels are best copies, one per song. A label never votes for itself
    # (held out); other copies of its song do learn from it.
    group_of = np.full(len(pending), -1, dtype=np.int64)
    group_of[label_rows] = np.arange(len(label_rows))

    votes: Dict[int, Vote] = {}
    test_score = {"labels": len(label_rows), "k": k}
    if has_audio:
        vectors = _normalise(np.vstack([np.asarray(pending[i].song.embedding, dtype=np.float32) for i in has_audio]))
        row_of = {i: r for r, i in enumerate(has_audio)}
        labels = vectors[[row_of[i] for i in label_rows]] if label_rows else np.zeros((0, vectors.shape[1]), np.float32)
        label_genres = np.array([gindex[pending[i].decision.genre] for i in label_rows], dtype=np.int64)
        label_groups = group_of[label_rows] if label_rows else np.zeros(0, np.int64)
        allowed = np.ones((len(has_audio), len(genre_ids)), dtype=bool)
        for r, i in enumerate(has_audio):
            fence = pending[i].fence
            if fence is not None:
                allowed[r] = False
                allowed[r, [gindex[g] for g in fence if g in gindex]] = True
        fenced = knn_votes(vectors, group_of[has_audio], allowed, labels, label_genres, label_groups, k)
        for r, i in enumerate(has_audio):
            votes[i] = _vote(fenced[r], genre_ids)

        # Test score: each trusted label held out, voting over every genre.
        if label_rows:
            free_rows = [r for r, i in enumerate(label_rows) if pending[i].fence is not None]
            plain = fenced[[row_of[i] for i in label_rows]]
            if free_rows:
                q = labels[free_rows]
                plain[free_rows] = knn_votes(
                    q, label_groups[free_rows], np.ones((len(free_rows), len(genre_ids)), dtype=bool),
                    labels, label_genres, label_groups, k,
                )
            test_score = _test_score(plain, label_genres, genre_ids, k)

    releases = _release_counts(pending)
    style_model = _style_model(cat, model_labels)
    decisions = []
    for i, p in enumerate(pending):
        d = p.decision if p.decision is not None else _second_pass(p, releases, votes.get(i))
        if votes.get(i) is not None:
            d.why["classifier"] = votes[i].as_dict()
            _flag(p, d, votes[i])
        if d.genre is not None:
            _pick_style(p, d, cat, style_model)
        decisions.append(d)
    return EngineResult(decisions, test_score, round(time.monotonic() - started, 3))


def _first_pass(song: SongInput, cat: gc.Catalog) -> _Pending:
    """Correction, strong sources and the fence."""
    views = source_views(song, cat)
    fence = fence_of(views)
    fence_list = sorted(fence) if fence is not None else None
    why: dict = {"sources": {name: v.as_dict() for name, v in views.items()}, "fence": fence_list}
    p = _Pending(song, views, fence, why)

    fixed = resolve_correction(song.correction, cat)
    if fixed is not None:
        genre, style = fixed
        why["correction"] = {"scope": song.correction_scope, "value": song.correction}
        p.decision = Decision(song.track_id, genre, style, CORRECTION, 1.0, fence=fence_list, why=why)
        if style is not None:
            why["style"] = {"style": style, "from": CORRECTION}
        return p

    answers = {name: views[name].answer for name in STRONG if name in views and views[name].answer is not None}
    named = set(answers.values())
    if GENRE_TAG in answers:
        p.tag_genre = answers[GENRE_TAG]
    if len(named) > 1:
        p.decision = _unsure(song, STRONG_DISAGREE, fence_list, why)
    elif fence is not None and not fence:
        p.decision = _unsure(song, EMPTY_FENCE, fence_list, why)
    elif named:
        genre = next(iter(named))
        if fence is not None and genre not in fence:
            p.decision = _unsure(song, OUTSIDE_FENCE, fence_list, why)
        else:
            why["agreeing"] = sorted(answers)
            p.decision = Decision(song.track_id, genre, None, STRONG_SOURCES, 1.0, fence=fence_list, why=why)
    return p


def _unsure(song: SongInput, reason: str, fence: Optional[List[str]], why: dict) -> Decision:
    return Decision(song.track_id, None, None, UNSURE, None, unsure_reason=reason, fence=fence, why=why)


def _release_counts(pending: Sequence[_Pending]) -> Dict[Any, Dict[str, int]]:
    """Genre counts of the decided songs (correction or strong sources) per release."""
    out: Dict[Any, Dict[str, int]] = {}
    for p in pending:
        d = p.decision
        if p.song.release_id is None or p.song.compilation or d is None or d.genre is None:
            continue
        counts = out.setdefault(p.song.release_id, {})
        counts[d.genre] = counts.get(d.genre, 0) + 1
    return out


def _second_pass(p: _Pending, releases: Dict[Any, Dict[str, int]], vote: Optional[Vote]) -> Decision:
    """No correction and no strong source: release lean, then the classifier."""
    song, fence = p.song, p.fence
    why = p.why
    fence_list = sorted(fence) if fence is not None else None

    counts = releases.get(song.release_id) if song.release_id is not None and not song.compilation else None
    if counts:
        decided = sum(counts.values())
        genre, n = max(counts.items(), key=lambda kv: (kv[1], kv[0]))
        share = n / decided
        why["release"] = {"genre": genre, "share": round(share, 3), "decided": decided}
        if decided >= RELEASE_MIN_DECIDED and share >= RELEASE_LEAN and (fence is None or genre in fence):
            if vote is not None and vote.genre not in (None, genre) and vote.share >= CLASSIFIER_OVERRULES:
                return _unsure(song, RELEASE_VS_CLASSIFIER, fence_list, why)
            return Decision(song.track_id, genre, None, RELEASE, round(share, 4), fence=fence_list, why=why)

    if vote is None:
        return _unsure(song, NO_AUDIO, fence_list, why)
    if vote.genre is None or vote.share < CLASSIFIER_DECIDES:
        return _unsure(song, CLASSIFIER_UNSURE, fence_list, why)
    return Decision(song.track_id, vote.genre, None, CLASSIFIER, round(vote.share, 4), fence=fence_list, why=why)


def _flag(p: _Pending, d: Decision, vote: Vote) -> None:
    """A classifier at 80% or more against a specific genre tag: keep the tag, review."""
    if (
        d.source == STRONG_SOURCES
        and p.tag_genre == d.genre
        and vote.genre not in (None, d.genre)
        and vote.share >= CLASSIFIER_OVERRULES
    ):
        d.review_reason = CLASSIFIER_DISAGREES


def _style_model(cat: gc.Catalog, model_labels: Optional[Sequence[str]]) -> List[Tuple[int, str, str]]:
    """(activation index, genre, style) for each Discogs400 label naming a style."""
    out = []
    for i, label in enumerate(model_labels or ()):
        t = cat.discogs400_label(label)
        if t is not None and t.genre is not None and t.style is not None:
            out.append((i, t.genre, t.style))
    return out


def _pick_style(p: _Pending, d: Decision, cat: gc.Catalog, model: List[Tuple[int, str, str]]) -> None:
    """A strong source's style inside the genre, else the Discogs400 model's."""
    if d.style is not None:
        return
    for name in STRONG:
        view = p.views.get(name)
        if view is None:
            continue
        for style in view.styles:
            s = cat.styles.get(style)
            if s is not None and s.genre == d.genre:
                d.style = style
                d.why["style"] = {"style": style, "from": name}
                return
    acts = p.song.activations
    if not model or acts is None:
        return
    best: Optional[Tuple[float, str]] = None
    for i, genre, style in model:
        if genre == d.genre and i < len(acts):
            a = float(acts[i])
            if best is None or a > best[0]:
                best = (a, style)
    if best is not None and best[0] >= STYLE_MIN_ACTIVATION:
        d.style = best[1]
        d.why["style"] = {"style": best[1], "from": "discogs400", "activation": round(best[0], 3)}


def _test_score(votes: np.ndarray, truth: np.ndarray, genre_ids: Sequence[str], k: int) -> dict:
    """Leave-one-out accuracy of the classifier on the trusted labels."""
    total = votes.sum(axis=1)
    voted = total > 0
    top = votes.argmax(axis=1)
    share = np.where(voted, votes.max(axis=1) / np.where(voted, total, 1), 0.0)
    right = voted & (top == truth)
    sure = voted & (share >= CLASSIFIER_DECIDES)

    def score(mask: np.ndarray) -> dict:
        n = int(mask.sum())
        n_sure = int((mask & sure).sum())
        return {
            "labels": n,
            "accuracy": round(float((mask & right).sum()) / n, 3) if n else None,
            "decides": round(n_sure / n, 3) if n else None,
            "accuracy_when_deciding": round(float((mask & sure & right).sum()) / n_sure, 3) if n_sure else None,
        }

    per_genre = {}
    for gi in sorted(set(truth.tolist()), key=lambda g: -int((truth == g).sum())):
        per_genre[genre_ids[gi]] = score(truth == gi)
    return {"k": k, "decides_at": CLASSIFIER_DECIDES, **score(np.ones(len(truth), dtype=bool)), "per_genre": per_genre}


# --- summaries ------------------------------------------------------------------


def summarize(result: EngineResult) -> dict:
    """Counts by decision source, unsure reason, review flag, genre and style."""
    by_source: Dict[str, int] = {}
    unsure: Dict[str, int] = {}
    genres: Dict[str, int] = {}
    styles = 0
    flagged = 0
    for d in result.decisions:
        by_source[d.source] = by_source.get(d.source, 0) + 1
        if d.unsure_reason:
            unsure[d.unsure_reason] = unsure.get(d.unsure_reason, 0) + 1
        if d.genre is not None:
            genres[d.genre] = genres.get(d.genre, 0) + 1
        if d.style is not None:
            styles += 1
        if d.review_reason:
            flagged += 1
    n = len(result.decisions)
    decided = n - by_source.get(UNSURE, 0)
    return {
        "tracks": n,
        "decided": decided,
        "unsure": n - decided,
        "with_style": styles,
        "flagged_for_review": flagged,
        "by_source": dict(sorted(by_source.items(), key=lambda kv: -kv[1])),
        "unsure_reasons": dict(sorted(unsure.items(), key=lambda kv: -kv[1])),
        "genres": dict(sorted(genres.items(), key=lambda kv: -kv[1])),
        "seconds": result.seconds,
    }


# --- database ---------------------------------------------------------------------

_TRACKS_SQL = """
    SELECT t.id, s.song_id, s.best_copy, a.name, t.title, t.genre_tag, t.release_id,
           COALESCE(r.is_compilation, FALSE), t.mb_recording_id, r.mb_release_id,
           r.mb_release_group_id, g.activations
    FROM tracks t
    JOIN songs s ON s.track_id = t.id
    LEFT JOIN artists a ON a.id = t.artist_id
    LEFT JOIN releases r ON r.id = t.release_id
    LEFT JOIN track_audio_genres g ON g.track_id = t.id
    ORDER BY t.id
"""


def _embedding(value) -> Optional[np.ndarray]:
    """A vector column read in binary: an array (pgvector adapters) or raw bytes."""
    if value is None:
        return None
    if isinstance(value, np.ndarray):
        return value.astype(np.float32, copy=False)
    if hasattr(value, "to_numpy"):
        return value.to_numpy().astype(np.float32, copy=False)
    raw = bytes(value)
    # pgvector binary format: dims (int16), unused (int16), float4 values.
    dims = int.from_bytes(raw[:2], "big")
    return np.frombuffer(raw, dtype=">f4", count=dims, offset=4).astype(np.float32)


def _embeddings(conn) -> Dict[Any, np.ndarray]:
    with conn.cursor(binary=True) as cur:
        cur.execute("SELECT track_id, embedding FROM track_audio_genres WHERE embedding IS NOT NULL")
        return {tid: _embedding(v) for tid, v in cur}


def _outside(conn) -> Dict[Tuple[str, str], Tuple[list, Optional[str]]]:
    rows = conn.execute("SELECT source, ref, genres, discogs_release_id FROM outside_genres WHERE found").fetchall()
    return {(r[0], r[1]): (r[2] or [], r[3]) for r in rows}


@dataclass
class Corrections:
    """Corrections by scope: song id, release id, artist name key."""

    songs: Dict[Any, str] = field(default_factory=dict)
    releases: Dict[Any, str] = field(default_factory=dict)
    artists: Dict[str, str] = field(default_factory=dict)

    def for_song(self, song_id, release_id, credit: Optional[str]) -> Tuple[Optional[str], Optional[str]]:
        """(value, scope) of the most specific correction: song, release, artist."""
        from musicbrainz import name_key, primary_artist

        if song_id in self.songs:
            return self.songs[song_id], SONG_SCOPE
        if release_id is not None and release_id in self.releases:
            return self.releases[release_id], RELEASE_SCOPE
        if credit:
            value = self.artists.get(name_key(credit)) or self.artists.get(name_key(primary_artist(credit)))
            if value:
                return value, ARTIST_SCOPE
        return None, None


def load_corrections(conn, cat: gc.Catalog) -> Corrections:
    """Corrections to a known genre or style; the newest wins when two land
    on one target. A correction to an unknown id is skipped, so it never
    hides a wider one."""
    from musicbrainz import name_key

    rows = conn.execute(
        """
        SELECT s.song_id, c.artist, c.release_id, c.genre
        FROM genre_corrections c LEFT JOIN songs s ON s.track_id = c.track_id
        ORDER BY c.created_at, c.id
        """
    ).fetchall()
    out = Corrections()
    for song_id, artist, release_id, genre in rows:
        if resolve_correction(genre, cat) is None:
            continue
        if artist is not None:
            out.artists[name_key(artist)] = genre
        elif release_id is not None:
            out.releases[release_id] = genre
        elif song_id is not None:
            out.songs[song_id] = genre
    return out


def load_songs(conn, cat: gc.Catalog) -> List[SongInput]:
    """Every track with its sources, from the database."""
    from genre_scoring import _mb_tags_by_key, artist_mb_tags
    from outside_genres import DISCOGS_RELEASE, LASTFM_TRACK, MB_RECORDING as OG_RECORDING
    from outside_genres import MB_RELEASE as OG_RELEASE
    from outside_genres import MB_RELEASE_GROUP, lastfm_ref

    outside = _outside(conn)
    vectors = _embeddings(conn)
    artists = _mb_tags_by_key(conn)
    fixes = load_corrections(conn, cat)

    def genres(source: str, ref: Optional[str]) -> list:
        return outside.get((source, ref), ([], None))[0] if ref else []

    songs = []
    for (track_id, song_id, best_copy, artist, title, genre_tag, release_id, compilation,
         recording_id, mb_release_id, mb_group_id, activations) in conn.execute(_TRACKS_SQL):
        release = outside.get((OG_RELEASE, mb_release_id)) if mb_release_id else None
        discogs_id = release[1] if release else None
        correction, scope = fixes.for_song(song_id, release_id, artist)
        songs.append(SongInput(
            track_id=track_id,
            song_id=song_id,
            best_copy=bool(best_copy),
            release_id=release_id,
            compilation=bool(compilation),
            genre_tag=genre_tag,
            discogs=genres(DISCOGS_RELEASE, discogs_id),
            mb_recording=genres(OG_RECORDING, recording_id),
            mb_release=genres(OG_RELEASE, mb_release_id) or genres(MB_RELEASE_GROUP, mb_group_id),
            artist_genres=artist_mb_tags(artist, artists) or (),
            lastfm=genres(LASTFM_TRACK, lastfm_ref(artist, title)) if artist and title else [],
            correction=correction,
            correction_scope=scope,
            activations=activations,
            embedding=vectors.get(track_id),
            artist=artist,
            title=title,
        ))
    return songs


def run(conn, model_labels: Optional[Sequence[str]] = None, cat: Optional[gc.Catalog] = None) -> EngineResult:
    """Load every song from the database and decide its genre (no writes)."""
    cat = cat or gc.catalog()
    started = time.monotonic()
    songs = load_songs(conn, cat)
    loaded = time.monotonic() - started
    result = decide(songs, cat, model_labels)
    result.loaded_seconds = round(loaded, 3)
    result.songs = songs
    log.info("genre engine: loaded %d tracks in %.2fs, decided in %.2fs", len(songs), loaded, result.seconds)
    return result


def write_decisions(conn, result: EngineResult, cat: Optional[gc.Catalog] = None) -> None:
    """Replace track_genre_decisions and the stored test score. The caller commits."""
    from psycopg.types.json import Jsonb

    cat = cat or gc.catalog()
    conn.execute("DELETE FROM track_genre_decisions")
    with conn.cursor() as cur:
        cur.executemany(
            """
            INSERT INTO track_genre_decisions
                (track_id, genre, style, source, confidence, unsure_reason, review_reason,
                 fence, why, catalog_version)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
            """,
            [
                (d.track_id, d.genre, d.style, d.source, d.confidence, d.unsure_reason,
                 d.review_reason, d.fence, Jsonb(d.why), cat.version)
                for d in result.decisions
            ],
        )
    conn.execute(
        """
        INSERT INTO app_state (key, value, updated_at) VALUES (%s, %s, NOW())
        ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
        """,
        (TEST_SCORE_KEY, Jsonb({**result.test_score, "catalog_version": cat.version})),
    )


def write_station_genres(conn, result: EngineResult) -> None:
    """Rewrite track_station_genres from the decisions: one row per decided
    song, none for unsure songs, so they join no genre station. The caller commits."""
    conn.execute("DELETE FROM track_station_genres")
    with conn.cursor() as cur:
        cur.executemany(
            """
            INSERT INTO track_station_genres (track_id, genre, score, is_primary, corrected)
            VALUES (%s, %s, %s, TRUE, %s)
            """,
            [
                (d.track_id, d.genre, d.confidence if d.confidence is not None else 0.0, d.source == CORRECTION)
                for d in result.decisions
                if d.genre is not None
            ],
        )


def score_library(conn, model_labels: Optional[Sequence[str]] = None) -> dict:
    """The genre rescore: catalog, decisions, station genres and test score.
    The caller commits."""
    cat = gc.catalog()
    gc.sync_catalog(conn, cat)
    result = run(conn, model_labels, cat)
    write_decisions(conn, result, cat)
    write_station_genres(conn, result)
    summary = summarize(result)
    log.info("genre engine: %s", {k: summary[k] for k in ("tracks", "decided", "unsure", "flagged_for_review")})
    return {"tracks": summary["tracks"], "scored": summary["decided"], "unsure": summary["unsure"]}


# --- bulk report (CLI, issue #138) -------------------------------------------------------


def agreement_with_v1(conn, result: EngineResult, cat: gc.Catalog) -> dict:
    """How often the new genre matches the old engine's primary genre (moved
    to the new ids), over the songs both decided."""
    rows = conn.execute("SELECT track_id, genre FROM track_station_genres WHERE is_primary").fetchall()
    old = {}
    id_map = cat.id_map()
    for track_id, genre in rows:
        old[track_id] = genre if genre in cat.genres else id_map.get(genre)
    both = agree = 0
    confusion: Dict[Tuple[str, str], int] = {}
    for d in result.decisions:
        was = old.get(d.track_id)
        if d.genre is None or was is None:
            continue
        both += 1
        if was == d.genre:
            agree += 1
        else:
            confusion[(was, d.genre)] = confusion.get((was, d.genre), 0) + 1
    return {
        "compared": both,
        "agreement": round(agree / both, 3) if both else None,
        "old_unsure_now": sum(1 for d in result.decisions if d.genre is None and old.get(d.track_id)),
        "top_changes": [
            {"old": a, "new": b, "tracks": n}
            for (a, b), n in sorted(confusion.items(), key=lambda kv: -kv[1])[:20]
        ],
    }


def examples(result: EngineResult, per_reason: int = 3) -> dict:
    """A few unsure and flagged songs per reason, with their "why"."""
    songs = {s.track_id: s for s in result.songs}
    out: Dict[str, list] = {}
    for d in result.decisions:
        key = d.unsure_reason or d.review_reason
        if not key or len(out.setdefault(key, [])) >= per_reason:
            continue
        s = songs.get(d.track_id)
        out[key].append({
            "track_id": str(d.track_id),
            "artist": s.artist if s else None,
            "title": s.title if s else None,
            "genre_tag": s.genre_tag if s else None,
            "genre": d.genre,
            "why": d.why,
        })
    return out


def run_report(conn, model_labels: Optional[Sequence[str]] = None, write: bool = False, per_reason: int = 3) -> dict:
    """Run the engine over the whole library and report counts and agreement.

    Read only unless write is set; then it stores track_genre_decisions and
    the test score (migration 036), but not track_station_genres, which the
    live stations read. The caller commits.
    """
    cat = gc.catalog()
    result = run(conn, model_labels, cat)
    report = {
        "catalog_version": cat.version,
        "summary": summarize(result),
        "loaded_seconds": result.loaded_seconds,
        "test_score": result.test_score,
        "agreement_with_v1": agreement_with_v1(conn, result, cat),
        "examples": examples(result, per_reason),
        "written": False,
    }
    if write:
        write_decisions(conn, result, cat)
        report["written"] = True
    return report
