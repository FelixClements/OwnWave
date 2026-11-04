import { useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { Pencil, RefreshCw, Trash2 } from 'lucide-react';
import { useAuth } from '@/lib/auth';
import { plural } from '@/lib/format';
import { useDeleteStation, usePlaylists, useQueue, useStation } from '@/lib/queries';
import { usePlayer } from '@/player/PlayerProvider';
import { HeroBody, HeroHeader, TrackList, stationCover, useArtColor, useStationGenreLabel } from '@/components/media';
import { StationEditor } from '@/components/StationEditor';
import { Art, Empty, ErrorText, IconButton, PlayButton, Spinner } from '@/components/ui';

/** The station page lists this many of the queue, not the whole pool. */
const PREVIEW_TRACKS = 100;

export function StationPage() {
  const { id = '' } = useParams();
  const { user } = useAuth();
  const station = useStation(id);
  const player = usePlayer();
  const preview = useQueue(player.station?.id === id ? null : id);
  const { data: playlists = [] } = usePlaylists();
  const genreLabel = useStationGenreLabel();
  const color = useArtColor(stationCover(id), station.data?.name ?? id);
  const del = useDeleteStation();
  const navigate = useNavigate();
  const [editing, setEditing] = useState(false);

  if (station.isLoading) return <Spinner />;
  if (!station.data) return <Empty title="Station not found">It may have been deleted or hidden.</Empty>;
  const s = station.data;
  const active = player.station?.id === s.id;
  const genreStation = s.kind === 'genre';
  const canChange = !s.shared || !!user?.is_admin;
  // An all-playlists station lists every playlist: say so instead.
  const names = s.all_playlists ? [] : s.playlist_ids.map((pid) => playlists.find((p) => p.id === pid)).filter(Boolean);
  const tracks = active ? [...(player.track ? [player.track] : []), ...player.upcoming] : (preview.data ?? []);
  const shown = Math.min(tracks.length, PREVIEW_TRACKS);

  return (
    <>
      <HeroHeader
        color={color}
        art={<Art src={stationCover(s.id)} name={s.name} className="aspect-square w-full" />}
        kind={genreStation ? 'Genre station' : s.shared ? 'Station' : 'Your station'}
        title={s.name}
        meta={
          <>
            <span className="font-semibold">{genreLabel(s)}</span>
            <span className="mx-1.5">•</span>
            {plural(s.track_count, 'track')}
            {s.all_playlists && (
              <>
                <span className="mx-1.5">•</span>
                <span>from every playlist</span>
              </>
            )}
            {names.length > 0 && (
              <>
                <span className="mx-1.5">•</span>
                from{' '}
                {names.map((p, i) => (
                  <span key={p!.id}>
                    {i > 0 && ', '}
                    <Link to={`/playlist/${p!.id}`} className="font-semibold hover:underline">
                      {p!.name}
                    </Link>
                  </span>
                ))}
              </>
            )}
          </>
        }
      />
      <HeroBody color={color}>
        <div className="flex items-center gap-4 py-6">
          <PlayButton
            playing={active && player.playing}
            label={active && player.playing ? 'Pause' : `Play ${s.name}`}
            onClick={() => (active ? player.toggle() : void player.playStation(s))}
          />
          {!active && (
            <IconButton label="Shuffle the preview" onClick={() => void preview.refetch()}>
              <RefreshCw className="h-5 w-5" />
            </IconButton>
          )}
          {canChange && (
            <>
              <IconButton label={genreStation ? 'Rename station' : 'Edit station'} onClick={() => setEditing(true)}>
                <Pencil className="h-5 w-5" />
              </IconButton>
              {/* Genre stations are hidden in Admin, not deleted. */}
              {!genreStation && (
                <IconButton
                  label="Delete station"
                  onClick={() => {
                    if (confirm(`Delete “${s.name}”${s.shared ? ' for everyone' : ''}?`)) del.mutate(s.id, { onSuccess: () => navigate('/stations') });
                  }}
                >
                  <Trash2 className="h-5 w-5" />
                </IconButton>
              )}
            </>
          )}
        </div>
        <ErrorText error={del.error ?? (active ? player.error : null)} />
        <h2 className="mb-2 mt-2 text-sm font-semibold text-muted">
          {active ? 'Now playing and up next' : 'Coming up when you press play (shuffled each time)'}
          {shown > 0 && shown < s.track_count && (
            <span className="font-normal">
              {' '}
              · showing {shown} of {plural(s.track_count, 'track')}
            </span>
          )}
        </h2>
        {preview.isLoading && !active ? (
          <Spinner />
        ) : (
          <TrackList
            tracks={tracks.slice(0, PREVIEW_TRACKS)}
            empty={<Empty title="No tracks yet">None of this station's playlist tracks are in your library with these genres. Rescan, or widen the genres.</Empty>}
          />
        )}
      </HeroBody>
      <StationEditor open={editing} onClose={() => setEditing(false)} station={s} />
    </>
  );
}
