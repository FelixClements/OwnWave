import { useEffect, useMemo, useState, useSyncExternalStore } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import clsx from 'clsx';
import { ChevronLeft, ChevronRight, Play, RefreshCw, Square, Undo2 } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
import { api, call } from '@/api/client';
import type { GenreCatalogEntry, GenreCorrection, GenreOverview, GenreReviewGroup, GenreReviewSong, GenreTestScore, GenreWhy } from '@/api/types';
import { plural, timeAgo } from '@/lib/format';
import {
  keys,
  useGenreCorrectionActions,
  useGenreCorrections,
  useGenreDecision,
  useGenreOverview,
  useGenreReview,
  useRefetchReleaseGenres,
  type GenreCorrectionBody,
} from '@/lib/queries';
import { streamUrl } from '@/player/PlayerProvider';
import { Block, Row, Section, Tag, table } from '@/components/settings-ui';
import { Button, ErrorText, IconButton, Spinner, Tabs } from '@/components/ui';

// The admin Genres page (issue #140): Overview, Review and Corrections.

type GenreTab = 'overview' | 'review' | 'corrections';
const TABS: GenreTab[] = ['overview', 'review', 'corrections'];

const pct = (v: number | null | undefined) => (v == null ? '–' : `${Math.round(v * 100)}%`);

/** Names of genres and styles, and each style's genre, from the catalog. */
type Catalog = {
  genres: GenreCatalogEntry[];
  name: (id: string | null | undefined) => string;
  stylesOf: (genre: string) => GenreCatalogEntry[];
};

function useCatalog(overview: GenreOverview | undefined): Catalog {
  return useMemo(() => {
    const names = new Map<string, string>();
    const styles = new Map<string, GenreCatalogEntry[]>();
    for (const g of overview?.genres ?? []) {
      names.set(g.id, g.name);
      styles.set(g.id, g.styles ?? []);
      for (const s of g.styles ?? []) names.set(s.id, s.name);
    }
    return {
      genres: overview?.genres ?? [],
      name: (id) => (id ? (names.get(id) ?? id.replace(/_/g, ' ')) : '–'),
      stylesOf: (genre) => styles.get(genre) ?? [],
    };
  }, [overview]);
}

export function GenresAdminPane() {
  const [params, setParams] = useSearchParams();
  const raw = params.get('tab');
  const tab: GenreTab = TABS.includes(raw as GenreTab) ? (raw as GenreTab) : 'overview';
  const overview = useGenreOverview();
  const catalog = useCatalog(overview.data);
  useEffect(() => stopPreview, []);
  return (
    <>
      <div className="mb-6">
        <Tabs
          label="Genres"
          value={tab}
          onChange={(t) => setParams(t === 'overview' ? {} : { tab: t }, { replace: true })}
          tabs={[
            { id: 'overview', label: 'Overview' },
            { id: 'review', label: 'Review', count: overview.data?.to_review },
            { id: 'corrections', label: 'Corrections' },
          ]}
        />
      </div>
      {overview.error != null && (
        <div className="mb-6">
          <ErrorText error={overview.error} />
        </div>
      )}
      {tab === 'overview' && <OverviewTab overview={overview.data} loading={overview.isLoading} catalog={catalog} />}
      {tab === 'review' && <ReviewTab catalog={catalog} />}
      {tab === 'corrections' && <CorrectionsTab catalog={catalog} />}
    </>
  );
}

// --- overview ---------------------------------------------------------------------

function Stat({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <div className="rounded-xl border border-line bg-card px-4 py-3">
      <div className="text-[12px] text-faint">{label}</div>
      <div className="mt-0.5 truncate text-xl font-semibold tabular-nums">{value}</div>
      {hint && <div className="text-[11px] text-faint">{hint}</div>}
    </div>
  );
}

const SOURCE_LABELS: Record<string, string> = {
  correction: 'Corrections',
  strong_sources: 'Strong sources',
  release: 'Release lean',
  classifier: 'Classifier',
  unsure: 'Unsure',
  blend: 'Genre blend',
};

function OverviewTab({ overview, loading, catalog }: { overview?: GenreOverview; loading: boolean; catalog: Catalog }) {
  const qc = useQueryClient();
  const [refreshing, setRefreshing] = useState(false);
  if (loading) return <Spinner />;
  if (!overview) return null;
  const score = overview.test_score;
  const genres = [...overview.genres].filter((g) => g.songs > 0).sort((a, b) => b.songs - a.songs);
  return (
    <>
      <div className="mb-8 grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Stat label="Songs" value={overview.songs.toLocaleString()} />
        <Stat label="With a genre" value={overview.decided.toLocaleString()} hint={overview.songs ? pct(overview.decided / overview.songs) : undefined} />
        <Stat label="Unsure" value={overview.unsure != null ? overview.unsure.toLocaleString() : '–'} />
        <Stat label="Classifier disagrees with tag" value={overview.flagged != null ? overview.flagged.toLocaleString() : '–'} />
      </div>
      <Section title="Test score">
        {score ? <TestScore score={score} at={overview.test_score_at} catalog={catalog} /> : <Block className="text-sm text-muted">No test score yet: it is measured at every genre rescore.</Block>}
      </Section>
      {overview.by_source && (
        <Section title="Decided by">
          <Block className="flex flex-wrap gap-x-6 gap-y-1 text-sm">
            {Object.entries(overview.by_source)
              .sort((a, b) => b[1] - a[1])
              .map(([source, n]) => (
                <span key={source}>
                  {SOURCE_LABELS[source] ?? source} <strong className="tabular-nums">{n.toLocaleString()}</strong>
                </span>
              ))}
          </Block>
        </Section>
      )}
      <Section title="Songs per genre and style">
        <table className={table.table}>
          <thead>
            <tr>
              <th className={table.th}>Genre</th>
              <th className={clsx(table.th, 'text-right')}>Songs</th>
              <th className={table.th}>Top styles</th>
            </tr>
          </thead>
          <tbody>
            {genres.map((g) => {
              const styles = (g.styles ?? []).filter((s) => s.songs > 0).sort((a, b) => b.songs - a.songs);
              return (
                <tr key={g.id} className="align-top">
                  <td className={clsx(table.td, 'font-medium')}>{g.name}</td>
                  <td className={clsx(table.td, 'text-right tabular-nums')}>{g.songs.toLocaleString()}</td>
                  <td className={clsx(table.td, 'text-muted')}>
                    {styles.length === 0
                      ? '–'
                      : styles
                          .slice(0, 4)
                          .map((s) => `${s.name} ${s.songs}`)
                          .join(' · ') + (styles.length > 4 ? ` · +${styles.length - 4} more` : '')}
                  </td>
                </tr>
              );
            })}
            {genres.length === 0 && (
              <tr>
                <td colSpan={3} className={clsx(table.td, 'text-muted')}>
                  No genres yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </Section>
      <Section title="Actions">
        <Row title="Rescore genres" description="Decide every song's genre again from corrections, tags, outside sources and the classifier.">
          <Button
            size="sm"
            loading={refreshing}
            onClick={async () => {
              setRefreshing(true);
              await call(api.POST('/api/admin/rebuild-genres')).catch(() => {});
              setRefreshing(false);
              setTimeout(() => {
                for (const key of [keys.genres, keys.genreOverview, keys.genreReview]) void qc.invalidateQueries({ queryKey: key });
              }, 5000);
            }}
          >
            <RefreshCw className="h-4 w-4" /> Rescore genres
          </Button>
        </Row>
        <Row
          title="Outside sources"
          description="MusicBrainz, Discogs and Last.fm: keys, how much of the library they match, and fetching all their genres again."
        >
          <Link to="/settings/outside-sources" className="text-sm font-medium text-accent hover:underline">
            Open
          </Link>
        </Row>
      </Section>
    </>
  );
}

function TestScore({ score, at, catalog }: { score: GenreTestScore; at?: string; catalog: Catalog }) {
  const perGenre = Object.entries(score.per_genre ?? {});
  return (
    <>
      <Block className="text-sm">
        <p>
          Holding out each of {plural(score.labels ?? 0, 'trusted label')}, the classifier names its genre <strong>{pct(score.accuracy)}</strong> of the
          time. It is sure enough to decide ({pct(score.decides_at)} of the vote) for <strong>{pct(score.decides)}</strong> of them, and then it is right{' '}
          <strong>{pct(score.accuracy_when_deciding)}</strong> of the time.
        </p>
        {at && <p className="mt-1 text-[12px] text-faint">Measured {timeAgo(at)}</p>}
      </Block>
      {perGenre.length > 0 && (
        <table className={table.table}>
          <thead>
            <tr>
              <th className={table.th}>Genre</th>
              <th className={clsx(table.th, 'text-right')}>Labels</th>
              <th className={clsx(table.th, 'text-right')}>Right</th>
              <th className={clsx(table.th, 'text-right')}>Decides</th>
              <th className={clsx(table.th, 'text-right')}>Right when deciding</th>
            </tr>
          </thead>
          <tbody>
            {perGenre.map(([genre, s]) => (
              <tr key={genre}>
                <td className={table.td}>{catalog.name(genre)}</td>
                <td className={clsx(table.td, 'text-right tabular-nums')}>{s.labels}</td>
                <td className={clsx(table.td, 'text-right tabular-nums')}>{pct(s.accuracy)}</td>
                <td className={clsx(table.td, 'text-right tabular-nums')}>{pct(s.decides)}</td>
                <td className={clsx(table.td, 'text-right tabular-nums')}>{pct(s.accuracy_when_deciding)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  );
}

// --- short preview --------------------------------------------------------------------

/** How long a review preview plays, from 30% into the song. */
const PREVIEW_SECONDS = 20;

const preview = { audio: null as HTMLAudioElement | null, track: null as string | null, timer: 0, listeners: new Set<() => void>() };

function emitPreview() {
  for (const l of preview.listeners) l();
}

export function stopPreview() {
  window.clearTimeout(preview.timer);
  preview.audio?.pause();
  if (preview.track !== null) {
    preview.track = null;
    emitPreview();
  }
}

function startPreview(trackId: string) {
  stopPreview();
  const audio = (preview.audio ??= new Audio());
  audio.onloadedmetadata = () => {
    if (Number.isFinite(audio.duration) && audio.duration > PREVIEW_SECONDS * 2) audio.currentTime = audio.duration * 0.3;
  };
  audio.onended = stopPreview;
  audio.src = streamUrl(trackId);
  try {
    void audio.play()?.catch(stopPreview);
  } catch {
    // no audio here (tests)
  }
  preview.track = trackId;
  preview.timer = window.setTimeout(stopPreview, PREVIEW_SECONDS * 1000);
  emitPreview();
}

function usePreviewing(): string | null {
  return useSyncExternalStore(
    (l) => {
      preview.listeners.add(l);
      return () => preview.listeners.delete(l);
    },
    () => preview.track,
  );
}

function PreviewButton({ song }: { song: GenreReviewSong }) {
  const playing = usePreviewing() === song.track_id;
  return (
    <IconButton size="sm" label={playing ? `Stop ${song.title}` : `Play ${PREVIEW_SECONDS} seconds of ${song.title}`} active={playing} onClick={() => (playing ? stopPreview() : startPreview(song.track_id))}>
      {playing ? <Square className="h-3.5 w-3.5" /> : <Play className="h-4 w-4" />}
    </IconButton>
  );
}

// --- review -------------------------------------------------------------------------------

const UNSURE_REASONS: Record<string, string> = {
  strong_sources_disagree: 'Strong sources disagree',
  empty_fence: 'Weak sources rule out every genre',
  outside_fence: 'Strong source outside its fence',
  release_vs_classifier: 'Release and classifier disagree',
  no_audio: 'Not analysed yet',
  classifier_unsure: 'Classifier not sure',
};

const PAGE_GROUPS = 20;

function ReviewTab({ catalog }: { catalog: Catalog }) {
  const [offset, setOffset] = useState(0);
  const q = useGenreReview(offset, true, PAGE_GROUPS);
  const page = q.data;
  if (q.isLoading) return <Spinner />;
  if (!page) return <ErrorText error={q.error} />;
  const last = Math.min(page.offset + page.groups.length, page.total);
  return (
    <>
      <p className="mb-6 text-[13px] text-muted">
        Grouped by release: songs listeners hid on a genre station first, then unsure songs, then songs whose tag the classifier disagrees with, then the
        least sure songs on stations. Every correction teaches the classifier, and the rescore that follows right away moves similar songs too.
      </p>
      <ErrorText error={q.error} />
      {page.groups.length === 0 && (
        <Section>
          <Block className="text-sm text-muted">Nothing to check.</Block>
        </Section>
      )}
      {page.groups.map((g) => (
        <ReviewGroup key={g.release_id ?? g.songs[0]?.track_id} group={g} catalog={catalog} />
      ))}
      {page.total > 0 && (
        <div className="flex items-center justify-between gap-3 text-sm text-muted">
          <span>
            {page.total > page.groups.length ? `Groups ${page.offset + 1}–${last} of ${page.total}` : plural(page.total, 'group')} · {plural(page.songs, 'song')}
          </span>
          <span className="flex gap-2">
            <Button size="sm" variant="ghost" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - PAGE_GROUPS))}>
              <ChevronLeft className="h-4 w-4" /> Previous
            </Button>
            <Button size="sm" variant="ghost" disabled={last >= page.total} onClick={() => setOffset(offset + PAGE_GROUPS)}>
              Next <ChevronRight className="h-4 w-4" />
            </Button>
          </span>
        </div>
      )}
    </>
  );
}

function ReviewGroup({ group, catalog }: { group: GenreReviewGroup; catalog: Catalog }) {
  const refetch = useRefetchReleaseGenres();
  const title = group.release_id ? (
    <>
      {group.release_title}
      <span className="font-normal text-muted"> · {group.compilation ? 'Compilation' : (group.release_artist ?? 'Unknown artist')}</span>
    </>
  ) : (
    <span className="text-muted">Song without a release</span>
  );
  return (
    <Section
      title={title}
      action={
        group.release_id && (
          <span className="flex items-center gap-2">
            {refetch.isSuccess && <span className="text-[12px] text-muted">Fetching…</span>}
            <Button size="sm" variant="ghost" loading={refetch.isPending} onClick={() => refetch.mutate(group.release_id!)}>
              <RefreshCw className="h-3.5 w-3.5" /> Refetch outside sources
            </Button>
          </span>
        )
      }
    >
      {refetch.error != null && (
        <Block>
          <ErrorText error={refetch.error} />
        </Block>
      )}
      {group.songs.map((s) => (
        <ReviewSong key={s.track_id} song={s} releaseId={group.release_id} catalog={catalog} />
      ))}
    </Section>
  );
}

type Scope = 'song' | 'release' | 'artist';

function songReason(song: GenreReviewSong): { text: string; tone: 'warn' | 'neutral' } | null {
  if (song.hidden_by > 0) return { text: `Hidden by ${plural(song.hidden_by, 'listener')}`, tone: 'warn' };
  if (song.unsure_reason) return { text: UNSURE_REASONS[song.unsure_reason] ?? song.unsure_reason.replace(/_/g, ' '), tone: 'warn' };
  if (song.review_reason === 'classifier_disagrees') return { text: 'Classifier disagrees with the tag', tone: 'warn' };
  if (song.confidence != null) return { text: `${song.source === 'release' ? 'Release lean' : 'Classifier'} ${pct(song.confidence)}`, tone: 'neutral' };
  return null;
}

function ReviewSong({ song, releaseId, catalog }: { song: GenreReviewSong; releaseId?: string; catalog: Catalog }) {
  const { save, remove } = useGenreCorrectionActions();
  const [genre, setGenre] = useState(song.genre ?? '');
  const [style, setStyle] = useState(song.style ?? '');
  const [scope, setScope] = useState<Scope>('song');
  const [why, setWhy] = useState(false);
  const [saved, setSaved] = useState<{ id: string; what: string } | null>(null);
  const styles = genre ? catalog.stylesOf(genre) : [];
  const reason = songReason(song);
  const scopes: { value: Scope; label: string }[] = [
    { value: 'song', label: 'This song' },
    ...(releaseId ? [{ value: 'release' as const, label: 'Its release' }] : []),
    ...(song.artist ? [{ value: 'artist' as const, label: `Everything by ${song.artist}` }] : []),
  ];
  const submit = (g: string, s: string) => {
    const target: Partial<GenreCorrectionBody> =
      scope === 'release' ? { release_id: releaseId } : scope === 'artist' ? { artist: song.artist } : { track_id: song.track_id };
    save.mutate(
      { ...target, genre: g, ...(s ? { style: s } : {}) },
      { onSuccess: (r) => r.saved_id && setSaved({ id: r.saved_id, what: `${catalog.name(s || g)} for ${scopes.find((x) => x.value === scope)?.label.toLowerCase()}` }) },
    );
  };
  const unchanged = song.genre != null && genre === song.genre && style === (song.style ?? '');
  return (
    <div className="px-4 py-3 text-sm">
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
        <PreviewButton song={song} />
        <span className="min-w-0 truncate font-medium">{song.title}</span>
        <span className="min-w-0 truncate text-muted">{song.artist ?? 'Unknown artist'}</span>
        <Tag tone={song.genre ? 'ok' : 'neutral'}>{song.genre ? catalog.name(song.genre) + (song.style ? ` · ${catalog.name(song.style)}` : '') : 'Unsure'}</Tag>
        {reason && <Tag tone={reason.tone}>{reason.text}</Tag>}
        <button type="button" className="text-[12px] text-muted hover:text-ink hover:underline" aria-expanded={why} onClick={() => setWhy(!why)}>
          {why ? 'Hide why' : 'Why?'}
        </button>
      </div>
      {why && <WhyBreakdown trackId={song.track_id} catalog={catalog} />}
      <div className="mt-2 flex flex-wrap items-center gap-2">
        <select aria-label={`Genre of ${song.title}`} value={genre} onChange={(e) => (setGenre(e.target.value), setStyle(''))} className={selectClass}>
          {!genre && <option value="">Pick a genre</option>}
          {catalog.genres.map((g) => (
            <option key={g.id} value={g.id}>
              {g.name}
            </option>
          ))}
        </select>
        {styles.length > 0 && (
          <select aria-label={`Style of ${song.title}`} value={style} onChange={(e) => setStyle(e.target.value)} className={selectClass}>
            <option value="">No style</option>
            {styles.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>
        )}
        {scopes.length > 1 && (
          <select aria-label={`Correct for`} value={scope} onChange={(e) => setScope(e.target.value as Scope)} className={selectClass}>
            {scopes.map((s) => (
              <option key={s.value} value={s.value}>
                {s.label}
              </option>
            ))}
          </select>
        )}
        <Button size="sm" variant={unchanged ? 'secondary' : 'primary'} loading={save.isPending} disabled={!genre} onClick={() => submit(genre, style)}>
          {unchanged ? 'Looks right' : 'Save'}
        </Button>
        {song.suggestion && (
          <Button size="sm" variant="ghost" disabled={save.isPending} onClick={() => submit(song.suggestion!, '')}>
            Accept {catalog.name(song.suggestion)}
          </Button>
        )}
      </div>
      {saved && (
        <p className="mt-2 flex items-center gap-2 text-[13px] text-muted" role="status">
          Saved {saved.what}; rescoring.
          <button
            type="button"
            className="inline-flex items-center gap-1 text-ink hover:underline"
            disabled={remove.isPending}
            onClick={() => remove.mutate(saved.id, { onSuccess: () => setSaved(null) })}
          >
            <Undo2 className="h-3.5 w-3.5" /> Undo
          </button>
        </p>
      )}
      <ErrorText error={save.error ?? remove.error} />
    </div>
  );
}

const selectClass = 'h-9 rounded-lg border border-line bg-raised px-2 text-sm text-ink outline-none focus:border-ink';

const SOURCE_NAMES: Record<string, string> = {
  genre_tag: 'Genre tag',
  discogs: 'Discogs',
  mb_recording: 'MusicBrainz recording',
  mb_release: 'MusicBrainz release',
  artist: "Artist's genres",
  lastfm: 'Last.fm',
  correction: 'Correction',
  discogs400: 'Audio model',
};
const sourceName = (s: string) => SOURCE_NAMES[s] ?? s.replace(/_/g, ' ');

/** The engine's "why": each source's answer and fence, the fence, release lean, classifier and style. */
function WhyBreakdown({ trackId, catalog }: { trackId: string; catalog: Catalog }) {
  const q = useGenreDecision(trackId, true);
  if (q.isLoading) return <p className="mt-2 text-[13px] text-muted">Loading…</p>;
  if (!q.data) return <ErrorText error={q.error} />;
  const d = q.data;
  const w: GenreWhy = d.why ?? {};
  const names = (ids?: string[] | null) => (ids && ids.length ? ids.map((id) => catalog.name(id)).join(', ') : 'none');
  const lines: [string, string][] = [];
  if (d.genre_tag) lines.push(['Tag in the file', d.genre_tag]);
  for (const [source, v] of Object.entries(w.sources ?? {})) {
    const parts = [v.genre ? catalog.name(v.genre) : null, v.styles?.length ? `styles ${v.styles.join(', ')}` : null, v.fence ? `fences to ${names(v.fence)}` : null];
    lines.push([sourceName(source), parts.filter(Boolean).join(' · ') || 'nothing usable']);
  }
  lines.push(['Fence', w.fence ? names(w.fence) : 'open (any genre)']);
  if (w.correction) lines.push(['Correction', `${w.correction.scope} correction to ${catalog.name(w.correction.value)}`]);
  if (w.agreeing?.length) lines.push(['Agreeing', w.agreeing.map(sourceName).join(', ')]);
  if (w.release) lines.push(['Release lean', `${catalog.name(w.release.genre)} on ${pct(w.release.share)} of ${plural(w.release.decided ?? 0, 'decided song')}`]);
  if (w.classifier) {
    const runnerUps = (w.classifier as unknown as { runner_ups?: [string, number][] }).runner_ups;
    const runners = (runnerUps ?? []).map((r) => `${catalog.name(String(r[0]))} ${pct(Number(r[1]))}`).join(', ');
    lines.push(['Classifier', `${w.classifier.genre ? catalog.name(w.classifier.genre) : 'no vote'} ${pct(w.classifier.share)}${runners ? ` (then ${runners})` : ''}`]);
  }
  if (w.style) lines.push(['Style', `${catalog.name(w.style.style)} from ${sourceName(w.style.from ?? '')}${w.style.activation != null ? ` (${pct(w.style.activation)})` : ''}`]);
  return (
    <dl className="mt-2 grid grid-cols-[max-content_1fr] gap-x-4 gap-y-0.5 rounded-md bg-raised px-3 py-2 text-[13px]">
      {lines.map(([k, v]) => (
        <div key={k} className="contents">
          <dt className="text-muted">{k}</dt>
          <dd>{v}</dd>
        </div>
      ))}
    </dl>
  );
}

// --- corrections ----------------------------------------------------------------------------

function correctionTarget(c: GenreCorrection): string {
  if (c.scope === 'release') return `${c.release_title ?? 'Release'} · ${c.release_artist ?? 'Unknown artist'}`;
  if (c.scope === 'artist') return `Everything by ${c.artist}`;
  return `${c.track_title} · ${c.track_artist ?? 'Unknown artist'}`;
}

function CorrectionsTab({ catalog }: { catalog: Catalog }) {
  const [search, setSearch] = useState('');
  const [q, setQ] = useState('');
  useEffect(() => {
    const t = window.setTimeout(() => setQ(search.trim()), 250);
    return () => window.clearTimeout(t);
  }, [search]);
  const corrections = useGenreCorrections(q);
  const { remove } = useGenreCorrectionActions();
  const list = corrections.data ?? [];
  return (
    <>
      <p className="mb-4 text-[13px] text-muted">
        A correction beats every other source and teaches the classifier; a song's correction wins over its release's, which wins over its artist's.
        Undoing one rescores right away.
      </p>
      <input
        type="search"
        aria-label="Search corrections"
        placeholder="Search songs, releases, artists, genres"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        className="mb-4 h-9 w-full rounded-lg border border-line bg-raised px-3 text-sm text-ink outline-none placeholder:text-faint focus:border-ink"
      />
      <Section title={`Corrections${list.length ? ` (${list.length})` : ''}`}>
        {(corrections.error ?? remove.error) != null && (
          <Block>
            <ErrorText error={corrections.error ?? remove.error} />
          </Block>
        )}
        {corrections.isLoading && (
          <Block>
            <Spinner />
          </Block>
        )}
        {corrections.data && list.length === 0 && <Block className="text-sm text-muted">{q ? 'Nothing matches.' : 'None yet.'}</Block>}
        {list.map((c) => {
          const what = correctionTarget(c);
          return (
            <div key={c.id} className="flex flex-wrap items-center gap-x-3 gap-y-1 px-4 py-2.5 text-sm">
              <Tag>{c.scope === 'song' ? 'Song' : c.scope === 'release' ? 'Release' : 'Artist'}</Tag>
              <span className="min-w-0 flex-1 truncate">{what}</span>
              <span className="shrink-0 font-medium">
                {catalog.name(c.genre)}
                {c.style && <span className="font-normal text-muted"> · {catalog.name(c.style)}</span>}
              </span>
              <span className="w-20 shrink-0 text-right text-[12px] text-faint">{timeAgo(c.created_at)}</span>
              <Button size="sm" variant="ghost" aria-label={`Undo the correction of ${what}`} disabled={remove.isPending} onClick={() => remove.mutate(c.id)}>
                <Undo2 className="h-3.5 w-3.5" /> Undo
              </Button>
            </div>
          );
        })}
      </Section>
    </>
  );
}
