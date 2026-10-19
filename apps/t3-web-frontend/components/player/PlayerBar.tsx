'use client';

import { useCallback, useEffect, useMemo } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api, coverUrl, type QueueTrack } from '@/lib/api';
import { keys, useFeedbackMutation, useQueue } from '@/lib/queries';
import { useStation } from '@/lib/station';
import { usePlayback } from '@/lib/playback/use-playback';
import { Cover } from '@/components/tracks/Cover';
import { formatBpm } from '@/components/tracks/AudioStats';
import { IconButton } from '@/components/ui/Button';
import { Badge } from '@/components/ui/Layout';
import { BanIcon, HeartIcon, PauseIcon, PlayIcon, SkipNextIcon, SkipPrevIcon } from '@/components/ui/Icons';
import { StreamSelect } from './StreamSelect';

/**
 * Bottom player: owns the two <audio> decks and the playback engine for the
 * playing station, plus transport, like/ban and stream format.
 */
export function PlayerBar() {
  const { nowPlaying, setNowPlaying, playingStation, isPlaying, setIsPlaying, stations, stream } = useStation();
  const qc = useQueryClient();
  const { data: queue = [] } = useQueue(playingStation);

  const recordPlay = useMutation((vars: { id: string; stationId?: string }) =>
    api.recordPlay(vars.id, vars.stationId),
  );
  const feedback = useFeedbackMutation(playingStation);

  const handleTrackChange = useCallback(
    (track: QueueTrack) => {
      recordPlay.mutate({ id: track.id, stationId: playingStation ?? undefined });
      setNowPlaying(track);
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps -- mutate is stable
    [playingStation, setNowPlaying],
  );

  const fetchNextQueue = useCallback(() => {
    if (!playingStation) return Promise.resolve([] as QueueTrack[]);
    return qc.fetchQuery(keys.queue(playingStation), () => api.getQueue(playingStation), { staleTime: 0 });
  }, [qc, playingStation]);

  const { audioARef, audioBRef, skipNext, skipPrev, removeTrack, isCrossfading } = usePlayback({
    stationId: playingStation,
    queue,
    isPlaying,
    format: stream.format,
    bitrate: stream.bitrate,
    onTrackChange: handleTrackChange,
    fetchNextQueue,
  });

  const isLiked = useMemo(() => !!queue.find((q) => q.id === nowPlaying?.id)?.liked, [queue, nowPlaying]);
  const stationName = stations.find((s) => s.id === playingStation)?.name;

  const skip = useCallback(() => {
    if (!isCrossfading()) skipNext();
  }, [isCrossfading, skipNext]);

  const ban = (track: QueueTrack) => {
    feedback.mutate({ id: track.id, feedback: 'ban' });
    removeTrack(track.id);
  };

  useEffect(() => {
    if (typeof window === 'undefined' || !('mediaSession' in navigator) || !nowPlaying) return;
    navigator.mediaSession.metadata = new MediaMetadata({
      title: nowPlaying.title,
      artist: nowPlaying.artist || undefined,
      album: nowPlaying.album || undefined,
      artwork: [{ src: coverUrl(nowPlaying.id), sizes: '512x512', type: 'image/jpeg' }],
    });
    navigator.mediaSession.setActionHandler('play', () => setIsPlaying(true));
    navigator.mediaSession.setActionHandler('pause', () => setIsPlaying(false));
    navigator.mediaSession.setActionHandler('nexttrack', skip);
    navigator.mediaSession.setActionHandler('previoustrack', skipPrev);
  }, [nowPlaying, setIsPlaying, skip, skipPrev]);

  return (
    <footer
      aria-label="Player"
      className="h-16 shrink-0 border-t border-line bg-surface grid grid-cols-[1fr_auto] md:grid-cols-[1fr_auto_1fr] items-center gap-3 md:gap-4 px-3 md:px-4"
    >
      <audio ref={audioARef} crossOrigin="use-credentials" className="hidden" />
      <audio ref={audioBRef} crossOrigin="use-credentials" className="hidden" />

      <div className="flex items-center gap-2.5 min-w-0">
        {nowPlaying ? (
          <>
            <Cover id={nowPlaying.id} title={nowPlaying.title} className="w-10 h-10 md:w-11 md:h-11" />
            <div className="min-w-0 leading-tight">
              <div className="truncate font-medium">{nowPlaying.title}</div>
              <div className="truncate text-[11.5px] text-muted">
                {nowPlaying.artist || 'Unknown artist'}
                {stationName && <span className="text-accent"> · {stationName}</span>}
              </div>
            </div>
          </>
        ) : (
          <span className="text-muted">Nothing playing</span>
        )}
        <p className="sr-only" aria-live="polite">
          {nowPlaying ? `Now playing: ${nowPlaying.title} by ${nowPlaying.artist || 'unknown artist'}` : ''}
        </p>
      </div>

      <div className="flex items-center gap-1 md:gap-2">
        {nowPlaying && (
          <IconButton
            label={isLiked ? 'Unlike' : 'Like'}
            aria-pressed={isLiked}
            onClick={() => feedback.mutate({ id: nowPlaying.id, feedback: 'like', remove: isLiked })}
            className={isLiked ? 'text-accent hover:text-accent' : ''}
          >
            <HeartIcon filled={isLiked} />
          </IconButton>
        )}
        <IconButton label="Previous track" onClick={skipPrev} disabled={!nowPlaying} className="hidden sm:inline-grid">
          <SkipPrevIcon />
        </IconButton>
        <button
          type="button"
          onClick={() => setIsPlaying(!isPlaying)}
          disabled={!playingStation}
          aria-label={isPlaying ? 'Pause' : 'Play'}
          className="grid place-items-center w-9 h-9 rounded-full bg-fg text-bg hover:scale-105 transition disabled:opacity-40 disabled:hover:scale-100"
        >
          {isPlaying ? <PauseIcon className="w-4 h-4" /> : <PlayIcon className="w-4 h-4 ml-0.5" />}
        </button>
        <IconButton label="Skip" onClick={skip} disabled={!nowPlaying}>
          <SkipNextIcon />
        </IconButton>
        {nowPlaying && (
          <IconButton label="Ban track" onClick={() => ban(nowPlaying)} className="text-danger hover:text-danger">
            <BanIcon />
          </IconButton>
        )}
      </div>

      <div className="hidden md:flex items-center justify-end gap-2 min-w-0">
        {nowPlaying && (
          <span className="text-muted tabular-nums truncate">
            {[nowPlaying.bpm > 0 && `${formatBpm(nowPlaying.bpm)} BPM`, nowPlaying.key].filter(Boolean).join(' · ')}
          </span>
        )}
        <StreamSelect />
        {stream.format === 'flac' && <Badge tone="accent">Lossless</Badge>}
      </div>
    </footer>
  );
}
