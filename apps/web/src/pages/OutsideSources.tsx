import { useState } from 'react';
import clsx from 'clsx';
import { AlertTriangle, RefreshCw } from 'lucide-react';
import type { OutsideGenreValue, OutsideSource, OutsideSourceMatch } from '@/api/types';
import { plural, timeAgo } from '@/lib/format';
import { useOutsideSources, useRefetchOutsideGenres, useTestOutsideSource, useUpdateOutsideSource } from '@/lib/queries';
import { Block, Note, Row, Section, Switch, Tag, table } from '@/components/settings-ui';
import { Button, ErrorText, Spinner } from '@/components/ui';

const STATE: Record<OutsideSource['state'], { label: string; tone: 'ok' | 'neutral' | 'warn' | 'bad' }> = {
  on: { label: 'On', tone: 'ok' },
  off: { label: 'Off', tone: 'neutral' },
  missing_key: { label: 'Missing key', tone: 'warn' },
  refused_key: { label: 'Key refused', tone: 'bad' },
};

const KEY_NAME: Record<string, string> = { discogs: 'Personal access token', lastfm: 'API key' };
const KEY_HELP: Record<string, string> = {
  discogs: 'discogs.com → Settings → Developers → Generate new token.',
  lastfm: 'last.fm/api/account/create (free for non-commercial use).',
};
const COVERAGE_HELP: Record<string, string> = {
  musicbrainz: 'Songs and releases with a MusicBrainz recording or release id in their tags.',
  discogs: 'Releases whose MusicBrainz release links to a Discogs release (and their songs).',
  lastfm: 'Songs with an artist and a title (matched exactly, no search).',
};

/** The API answers errors with the response body; FastAPI's is {"detail": "..."}. */
function errorMessage(error: unknown): string {
  const text = error instanceof Error ? error.message : String(error);
  try {
    const detail = (JSON.parse(text) as { detail?: unknown }).detail;
    if (typeof detail === 'string') return detail;
  } catch {
    // plain text
  }
  return text;
}

const pct = (n: number, of: number) => (of > 0 ? `${Math.round((n / of) * 100)}%` : '–');

export function OutsideSourcesPane() {
  const sources = useOutsideSources();
  const refetch = useRefetchOutsideGenres();
  const data = sources.data;
  return (
    <>
      <Section title="Outside sources">
        <Block>
          <p className="text-sm text-muted">
            Genres from MusicBrainz, Discogs and Last.fm, looked up by ID only and cached
            {data ? ` for ${plural(data.max_age_days, 'day')}` : ''}. A switch or key saved here overrides the server&apos;s
            environment; with nothing saved, the environment decides.
          </p>
        </Block>
        <Row
          title="Fetch outside genres again"
          description="Look up every song's MusicBrainz, Discogs and Last.fm genres again in the background. Otherwise they are refreshed once they are old."
        >
          {refetch.isSuccess && <span className="text-[13px] text-muted">Queued</span>}
          <Button size="sm" loading={refetch.isPending} onClick={() => refetch.mutate()}>
            <RefreshCw className="h-4 w-4" /> Fetch again
          </Button>
        </Row>
        {refetch.error && (
          <Block>
            <ErrorText error={refetch.error} />
          </Block>
        )}
      </Section>
      {sources.isLoading ? (
        <Spinner />
      ) : sources.error ? (
        <ErrorText error={sources.error} />
      ) : (
        data?.sources.map((s) => <SourceSection key={s.source} source={s} library={data.library} />)
      )}
      <Note>Saved keys are encrypted on the server and never shown again: only where they come from and their last 4 characters.</Note>
    </>
  );
}

function SourceSection({ source: s, library }: { source: OutsideSource; library: { songs: number; releases: number } }) {
  const update = useUpdateOutsideSource();
  const state = STATE[s.state];
  return (
    <Section
      title={
        <span className="flex flex-wrap items-center gap-2">
          {s.name}
          <Tag tone={state.tone}>{state.label}</Tag>
          {s.running && <Tag tone="ok">Fetching</Tag>}
          {s.queued && !s.running && <Tag>Queued</Tag>}
        </span>
      }
    >
      <Row
        title="Switched on"
        description={
          s.enabled_from === 'page' ? 'Set on this page.' : `From the environment (${s.env_switch}).`
        }
      >
        {s.enabled_from === 'page' && (
          <Button size="sm" variant="ghost" disabled={update.isPending} onClick={() => update.mutate({ source: s.source, enabled: null })}>
            Use environment
          </Button>
        )}
        <Switch
          label={`${s.name} on`}
          checked={s.enabled}
          disabled={update.isPending}
          onChange={(enabled) => update.mutate({ source: s.source, enabled })}
        />
      </Row>
      {update.error && (
        <Block>
          <p role="alert" className="text-sm text-danger">
            {errorMessage(update.error)}
          </p>
        </Block>
      )}
      {s.key ? <KeyRow source={s} /> : <TestRow source={s} />}
      <Block>
        <LastRun source={s} />
      </Block>
      <Block>
        <MatchTable source={s} library={library} />
      </Block>
      <Block>
        <GenreYield source={s} />
      </Block>
    </Section>
  );
}

function keyDescription(s: OutsideSource): string {
  const k = s.key!;
  if (!k.set) return `Not set. Set it here or in ${k.env_var}.`;
  const end = k.last4 ? `, ends in ${k.last4}` : '';
  return k.source === 'page' ? `Saved on this page${end}.` : `From the environment (${k.env_var})${end}.`;
}

function KeyRow({ source: s }: { source: OutsideSource }) {
  const update = useUpdateOutsideSource();
  const test = useTestOutsideSource();
  const [value, setValue] = useState('');
  const name = KEY_NAME[s.source] ?? 'Key';
  const typed = value.trim();
  return (
    <>
      <Row title={name} description={keyDescription(s)}>
        {s.key?.source === 'page' && (
          <Button
            size="sm"
            variant="ghost"
            disabled={update.isPending}
            onClick={() => update.mutate({ source: s.source, key: null }, { onSuccess: () => test.reset() })}
          >
            Remove
          </Button>
        )}
      </Row>
      {s.key?.unreadable && (
        <Block>
          <p role="alert" className="flex items-start gap-2 text-sm text-danger">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
            The key saved here can no longer be read (the server&apos;s encryption key changed). Save it again.
          </p>
        </Block>
      )}
      <Block>
        <form
          className="flex flex-wrap items-center gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            if (!typed) return;
            update.mutate(
              { source: s.source, key: typed },
              {
                onSuccess: () => {
                  setValue('');
                  test.reset();
                },
              },
            );
          }}
        >
          <input
            type="password"
            aria-label={`New ${s.name} ${name.toLowerCase()}`}
            placeholder={s.key?.set ? `Replace the ${name.toLowerCase()}` : `Paste the ${name.toLowerCase()}`}
            autoComplete="off"
            spellCheck={false}
            value={value}
            onChange={(e) => setValue(e.target.value)}
            className="h-9 min-w-0 flex-1 basis-56 rounded-lg border border-line bg-raised px-2.5 text-sm text-ink outline-none focus:border-ink"
          />
          <Button size="sm" type="submit" disabled={!typed} loading={update.isPending} aria-label={`Save the ${s.name} key`}>
            Save
          </Button>
          <Button
            size="sm"
            type="button"
            loading={test.isPending}
            disabled={!typed && !s.key?.set}
            onClick={() => test.mutate({ source: s.source, key: typed || null })}
            aria-label={typed ? `Test this ${s.name} key` : `Test the ${s.name} key`}
          >
            {typed ? 'Test this key' : 'Test'}
          </Button>
        </form>
        <p className="mt-1.5 text-[12px] text-faint">{KEY_HELP[s.source]}</p>
        <TestResult test={test} />
        {update.error && (
          <p role="alert" className="mt-2 text-sm text-danger">
            {errorMessage(update.error)}
          </p>
        )}
      </Block>
    </>
  );
}

function TestRow({ source: s }: { source: OutsideSource }) {
  const test = useTestOutsideSource();
  return (
    <>
      <Row title="Connection" description={`Sends one small request to ${s.name}. Needs no key.`}>
        <Button size="sm" loading={test.isPending} onClick={() => test.mutate({ source: s.source })} aria-label={`Test ${s.name}`}>
          Test
        </Button>
      </Row>
      {(test.data || test.error) && (
        <Block>
          <TestResult test={test} />
        </Block>
      )}
    </>
  );
}

function TestResult({ test }: { test: ReturnType<typeof useTestOutsideSource> }) {
  if (test.error) {
    return (
      <p role="alert" className="mt-2 text-sm text-danger">
        {errorMessage(test.error)}
      </p>
    );
  }
  if (!test.data) return null;
  return (
    <p role="status" className={clsx('mt-2 text-sm', test.data.ok ? 'text-accent' : 'text-danger')}>
      {test.data.ok ? 'OK: ' : 'Failed: '}
      {test.data.message}
    </p>
  );
}

const RUN_STATUS: Record<string, string> = {
  ok: 'finished',
  refused_key: 'stopped: the key was refused',
  failed: 'stopped after repeated failures',
};

function LastRun({ source: s }: { source: OutsideSource }) {
  const run = s.last_run;
  return (
    <div className="space-y-0.5 text-[13px] text-muted">
      <div>
        <span className="text-faint">Last run: </span>
        {run ? (
          <>
            {run.at ? timeAgo(run.at) : 'unknown time'}, {RUN_STATUS[run.status] ?? run.status}. {plural(run.requests, 'request')},{' '}
            {plural(run.stored, 'lookup')} stored{run.failures ? `, ${plural(run.failures, 'failed lookup')}` : ''}
            {run.more ? '; more runs follow' : ''}.
          </>
        ) : (
          'none yet.'
        )}
      </div>
      <div>
        <span className="text-faint">Last refresh: </span>
        {s.last_refresh ? timeAgo(s.last_refresh) : 'never'}
        <span className="text-faint"> · up to {s.requests_per_run} requests per run</span>
      </div>
    </div>
  );
}

function MatchTable({ source: s, library }: { source: OutsideSource; library: { songs: number; releases: number } }) {
  const rows: [string, OutsideSourceMatch, number][] = [
    ['Songs', s.songs, library.songs],
    ['Releases', s.releases, library.releases],
  ];
  const num = clsx(table.th, 'text-right');
  return (
    <>
      <div className="mb-1 text-sm font-medium">Coverage and matches</div>
      <p className="mb-2 text-[12px] text-faint">{COVERAGE_HELP[s.source]} Found, not found and due are shares of what can be looked up.</p>
      <div className="-mx-4 overflow-x-auto">
        <table className={table.table}>
          <thead>
            <tr>
              <th className={table.th} />
              <th className={num}>Can look up</th>
              <th className={num}>Found</th>
              <th className={num}>Not found</th>
              <th className={num}>Still due</th>
            </tr>
          </thead>
          <tbody>
            {rows.map(([label, m, total]) => (
              <tr key={label}>
                <td className={table.td}>{label}</td>
                <Cell n={m.covered} share={pct(m.covered, total)} />
                <Cell n={m.found} share={pct(m.found, m.covered)} />
                <Cell n={m.not_found} share={pct(m.not_found, m.covered)} />
                <Cell n={m.due} share={pct(m.due, m.covered)} />
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {s.last_run && s.last_run.failures > 0 && (
        <p className="mt-2 text-[12px] text-faint">
          Failed in the last run: {plural(s.last_run.failures, 'lookup')} (they stay due and are tried again).
        </p>
      )}
    </>
  );
}

function Cell({ n, share }: { n: number; share: string }) {
  return (
    <td className={clsx(table.td, 'text-right tabular-nums')}>
      {n.toLocaleString()} <span className="text-[12px] text-faint">{share}</span>
    </td>
  );
}

function GenreYield({ source: s }: { source: OutsideSource }) {
  const y = s.genre_yield;
  const [showUntranslated, setShowUntranslated] = useState(false);
  return (
    <>
      <div className="mb-1 text-sm font-medium">Genre yield</div>
      <p className="text-[13px] text-muted">
        {y.with_genres.toLocaleString()} of {plural(y.found_rows, 'found lookup')} carry genres ({pct(y.with_genres, y.found_rows)}).{' '}
        {plural(y.values, 'distinct value')}
        {y.translated_values != null && (
          <>
            : {y.translated_values.toLocaleString()} the genre catalog translates, {y.untranslated_values?.toLocaleString()} it doesn&apos;t
          </>
        )}
        .
      </p>
      {y.top.length > 0 && <ValueList title="Most common" values={y.top} />}
      {y.untranslated.length > 0 && (
        <div className="mt-2">
          <Button size="sm" variant="ghost" onClick={() => setShowUntranslated((v) => !v)} aria-expanded={showUntranslated}>
            {showUntranslated ? 'Hide' : 'Show'} the most common values the catalog can&apos;t translate
          </Button>
          {showUntranslated && <ValueList values={y.untranslated} />}
        </div>
      )}
    </>
  );
}

function ValueList({ title, values }: { title?: string; values: OutsideGenreValue[] }) {
  return (
    <div className="mt-2">
      {title && <div className="mb-1 text-[12px] text-faint">{title}</div>}
      <ul className="flex flex-wrap gap-1.5">
        {values.map((v) => (
          <li
            key={`${v.kind}:${v.name}`}
            className="rounded-md border border-line px-2 py-0.5 text-[12px]"
            title={v.translated_to ? `Translates to ${v.translated_to}` : 'Not translated'}
          >
            {v.name}
            {v.kind !== 'genre' && <span className="text-faint"> ({v.kind})</span>}
            <span className="tabular-nums text-faint"> ×{v.rows.toLocaleString()}</span>
            {v.translated_to ? <span className="text-accent"> → {v.translated_to}</span> : <span className="text-danger"> ✕</span>}
          </li>
        ))}
      </ul>
    </div>
  );
}
