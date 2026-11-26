# 0008: Genres come from strong sources, fences and a library classifier

Status: accepted, 2026-11-24. Replaces the weighted genre blend (audio 0.5,
tags 0.35, MusicBrainz 0.15) and partly supersedes
[ADR 0002](0002-library-genre-stations.md) decision 5 (how corrections apply
and how the review list ranks).

## Context

Genre stations played songs from the wrong genre, such as metal songs on
Techno. On the live library (7,846 songs, 2026-11-24) 80% of songs have a genre
tag, but most tags are broad: `Pop/Rock` 953, `Electronic` 818, `Dance` 255.
About 190 of the 355 Techno songs had no tag or only a broad one, so the
Discogs400 audio model decided them almost alone. Holding out each of the 2,240
songs whose tag names one genre, that model's genre mapping matched the tag 51%
of the time (Techno 35%, mostly called Trance; Pop 24%, often called House).
Nearest neighbours on the same audio embeddings, voting with the library's own
labelled songs, matched 66%, and 86% when at least 60% of the vote agreed.

## Decision

1. **Two levels.** Each song gets one genre (House) and at most one style (Deep
   House). Styles are Discogs styles. Genres are a curated list of style groups,
   rebuilt from the library's counts; old genre ids in user data are mapped to
   the new ones once. Genres and styles with enough songs get stations; a genre
   page lists its style stations. Secondary genres are gone.
2. **Strong sources decide, weak sources fence.** Strong: a correction, a
   specific genre tag, a Discogs style, a MusicBrainz recording or release
   genre. Weak: broad tags, artist genres, Last.fm tags; they only narrow the
   set of genres a song may get. A correction always wins. Agreeing strong
   sources decide; disagreeing ones leave the song unsure.
3. **Then the release, then the classifier.** A song with no strong source
   takes its release's genre when at least 70% of the release agrees
   (compilations excluded). Otherwise a nearest-neighbour classifier over the
   audio embeddings decides, inside the fence, only at 60% vote or more.
   Classifier labels are corrections, specific tags and ID-matched Discogs or
   MusicBrainz genres; broad tags never teach it.
4. **Unsure is allowed.** A song nobody is sure about joins no genre station
   and waits on the admin review queue. A classifier at 80% or more against a
   specific tag keeps the tag but queues the song for review. The queue is
   grouped by release and ranked: songs listeners hid on a genre station, then
   unsure songs, then classifier-versus-tag disagreements, then the least sure
   songs on stations. A correction applies to a song, a release or an artist;
   "Looks right" saves the current genre as one.
5. **Outside sources match by ID only.** MusicBrainz by recording and release
   ids, Discogs through MusicBrainz links, Last.fm by exact artist and title.
   No name search. Results are cached per release and recording.
6. **One switch.** The new engine replaces the old one in one release, with no
   side-by-side period. The genre list is proposed from a run on a copy of the
   database and approved before that release. The switch is migration
   `038_genre_v2_switch` (#141): it adds genre kinds and styles to
   `station_genres` and moves stored v1 genre ids to the new genres; the code
   no longer checks whether it has run.

## Considered options

- Tuning the blend's weights: any weight on the Discogs400 mapping lets it
  overrule broad tags, which is where the wrong genres came from.
- A larger audio model (MAEST, MuQ): heavier, still a fixed taxonomy, and no
  measured gain on this library. Revisit only if the test score points there.
- Name search for outside sources: the files carry ISRCs (95% in a sample) and
  MusicBrainz ids, so IDs cost little coverage and avoid wrong-artist matches.

## Consequences

- Fewer songs reach genre stations at first; unsure songs need review.
- Every correction becomes a classifier label, so fixes spread to similar songs.
- Scans store release ids, album artists, ISRCs and MusicBrainz ids, which the
  old schema did not have.
