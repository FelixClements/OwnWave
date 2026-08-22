'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { QueueTrack } from '@/server/routers/app';
import { getCoverUrl } from '@/lib/api';
import { useStation } from '@/lib/station';
import { trpc } from '@/lib/trpc/client';
import { usePlayback } from '@/lib/playback/use-playback';
import type { StreamFormat } from '@/lib/playback/types';

function PlayIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 24 24"
      fill="currentColor"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path d="M8 5v14l11-7L8 5z" />
    </svg>
  );
}

function ThumbUpIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 24 24"
      fill="currentColor"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path d="M14 9V5a3 3 0 0 0-3-3l-4 9v11h11.28a2 2 0 0 0 2-1.7l1.38-9a2 2 0 0 0-2-2.3H14zM7 22H4a2 2 0 0 1-2-2v-9a2 2 0 0 1 2-2h3v13z" />
    </svg>
  );
}

function PauseIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 24 24"
      fill="currentColor"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z" />
    </svg>
  );
}

export function Player({ queue: queueProp }: { queue: QueueTrack[] }) {
  const { nowPlaying, setNowPlaying, selectedStation, setPlayingStation, playingStation, isPlaying, setIsPlaying, stations } = useStation();

  const [coverUrl, setCoverUrl] = useState<string | null>(null);
  const [coverError, setCoverError] = useState(false);
  const [format, setFormat] = useState<StreamFormat>('flac');
  const [bitrate, setBitrate] = useState('320');

  const recordPlay = trpc.recordPlay.useMutation();
  const utils = trpc.useContext();

  const handleTrackChange = useCallback(
    (fullTrack: QueueTrack, _index: number) => {
      const stationId = playingStation ?? selectedStation;
      setCoverUrl(getCoverUrl(fullTrack.id));
      setCoverError(false);
      recordPlay.mutate({ id: fullTrack.id, stationId: stationId ?? undefined });
      setNowPlaying(fullTrack);
      if (stationId) {
        setPlayingStation(stationId);
      }
    },
    [playingStation, selectedStation, recordPlay, setNowPlaying, setPlayingStation],
  );

  const {
    audioARef,
    audioBRef,
    queue,
    skipNext: engineSkipNext,
    skipPrev,
    isCrossfading,
  } = usePlayback({
    queue: queueProp,
    selectedStation,
    playingStation,
    isPlaying,
    format,
    bitrate,
    nowPlaying,
    onTrackChange: handleTrackChange,
  });

  const recordFeedback = trpc.recordFeedback.useMutation({
    onMutate: async (input) => {
      if (!playingStation) return {};
      await utils.queue.cancel({ id: playingStation });
      const previous = utils.queue.getData({ id: playingStation });
      utils.queue.setData({ id: playingStation }, (old) => {
        if (!old) return old;
        if (input.feedback === 'like') {
          return old.map((t) => (t.id === input.id ? { ...t, liked: true } : t));
        }
        if (input.feedback === 'skip') {
          const idx = old.findIndex((t) => t.id === input.id);
          if (idx === -1) return old;
          const moved = old[idx];
          const rest = old.filter((_, i) => i !== idx);
          return [...rest, moved];
        }
        if (input.feedback === 'ban') {
          return old.filter((t) => t.id !== input.id);
        }
        return old;
      });
      return { previous };
    },
    onError: (err, input, context) => {
      if (!playingStation) return;
      const previous = (context as any)?.previous;
      if (previous) {
        utils.queue.setData({ id: playingStation }, previous);
      }
    },
  });
  const removeFeedback = trpc.removeFeedback.useMutation({
    onMutate: async (input) => {
      if (input.feedback !== 'like' || !playingStation) return {};
      await utils.queue.cancel({ id: playingStation });
      const previous = utils.queue.getData({ id: playingStation });
      utils.queue.setData({ id: playingStation }, (old) => {
        if (!old) return old;
        return old.map((t) => (t.id === input.id ? { ...t, liked: false } : t));
      });
      return { previous };
    },
    onError: (err, input, context) => {
      if (input.feedback !== 'like' || !playingStation) return;
      const previous = (context as any)?.previous;
      if (previous) {
        utils.queue.setData({ id: playingStation }, previous);
      }
    },
    onSuccess: () => {
      if (playingStation) {
        utils.queue.invalidate({ id: playingStation });
      }
    },
  });

  const isLiked = useMemo(
    () => queue.find((q) => q.id === nowPlaying?.id)?.liked,
    [queue, nowPlaying]
  );

  useEffect(() => {
    const savedFormat = localStorage.getItem('ownwave:format');
    const savedBitrate = localStorage.getItem('ownwave:bitrate');
    if (savedFormat) setFormat(savedFormat as StreamFormat);
    if (savedBitrate) setBitrate(savedBitrate);
  }, []);

  useEffect(() => {
    localStorage.setItem('ownwave:format', format);
    localStorage.setItem('ownwave:bitrate', bitrate);
  }, [format, bitrate]);

  useEffect(() => {
    if (typeof window === 'undefined' || !('mediaSession' in navigator) || !nowPlaying) return;

    const artwork: MediaImage[] = [];
    if (coverUrl && !coverError) {
      artwork.push({
        src: coverUrl,
        sizes: '512x512',
        type: 'image/jpeg',
      });
    }

    navigator.mediaSession.metadata = new MediaMetadata({
      title: nowPlaying.title,
      artist: nowPlaying.artist || undefined,
      album: nowPlaying.album || undefined,
      artwork,
    });

    navigator.mediaSession.setActionHandler('play', () => setIsPlaying(true));
    navigator.mediaSession.setActionHandler('pause', () => setIsPlaying(false));
    navigator.mediaSession.setActionHandler('nexttrack', () => skipNext());
    navigator.mediaSession.setActionHandler('previoustrack', () => skipPrev());
  }, [nowPlaying, coverUrl, coverError, setIsPlaying, skipPrev]);

  useEffect(() => {
    if (typeof window === 'undefined' || !queue.length) return;
    try {
      localStorage.setItem('ownwave:offlineQueue', JSON.stringify(queue.slice(0, 20)));
    } catch {
      // ignore storage errors
    }
  }, [queue]);

  function skipNext() {
    if (isCrossfading() || !nowPlaying) return;
    recordFeedback.mutate({ id: nowPlaying.id, feedback: 'skip' });
    engineSkipNext();
  }

  function togglePlay() {
    setIsPlaying(!isPlaying);
  }

  return (
    <div className="w-full h-full relative flex items-center justify-between gap-2 md:gap-4 px-2 md:px-4">
      <audio ref={audioARef} crossOrigin="anonymous" className="hidden" />
      <audio ref={audioBRef} crossOrigin="anonymous" className="hidden" />

      {nowPlaying ? (
        <div className="flex items-center gap-2 md:gap-4 w-5/12 md:w-5/12 min-w-0">
          <div className="w-10 h-10 md:w-14 md:h-14 shrink-0 relative rounded shadow overflow-hidden bg-spotify-card flex items-center justify-center text-xs text-spotify-subdued font-bold">
            {coverUrl && !coverError ? (
              <img
                src={coverUrl}
                alt=""
                className="w-full h-full object-cover"
                onError={() => setCoverError(true)}
              />
            ) : (
              nowPlaying.title.charAt(0).toUpperCase()
            )}
          </div>
          <div className="min-w-0">
            <div className="text-xs md:text-sm font-bold text-spotify-text truncate">
              {nowPlaying.title}
            </div>
            <div className="text-[10px] md:text-xs text-spotify-subdued truncate">
              {nowPlaying.artist || 'Unknown artist'}
            </div>
            {(() => {
              const stationId = playingStation || selectedStation;
              const station = stations.find((s) => s.id === stationId);
              return station ? (
                <div className="text-[10px] md:text-xs text-spotify-green truncate">
                  {station.name}
                </div>
              ) : null;
            })()}
          </div>
        </div>
      ) : (
        <div className="w-5/12 md:w-5/12" />
      )}

      <div className="absolute left-1/2 -translate-x-1/2 flex items-center gap-2 md:gap-4">
        {nowPlaying ? (
          <button
            onClick={togglePlay}
            className="w-10 h-10 md:w-12 md:h-12 rounded-full bg-spotify-text text-spotify-bg flex items-center justify-center hover:scale-105 transition disabled:opacity-50"
            aria-label={isPlaying ? 'Pause' : 'Play'}
          >
            {isPlaying ? (
              <PauseIcon className="w-5 h-5 md:w-6 md:h-6" />
            ) : (
              <PlayIcon className="w-5 h-5 md:w-6 md:h-6 ml-0.5" />
            )}
          </button>
        ) : null}
        {nowPlaying && (
          <div className="flex items-center gap-1 md:gap-2">
            <button
              onClick={() =>
                isLiked
                  ? removeFeedback.mutate({ id: nowPlaying.id, feedback: 'like' })
                  : recordFeedback.mutate({ id: nowPlaying.id, feedback: 'like' })
              }
              className={`px-2.5 py-1.5 rounded transition text-xs flex items-center gap-1 ${
                isLiked
                  ? 'bg-spotify-green text-black'
                  : 'bg-spotify-elevated text-spotify-text hover:bg-spotify-card-hover'
              }`}
              title={isLiked ? 'Remove like' : 'Like'}
            >
              <ThumbUpIcon className="w-3.5 h-3.5" />
              {isLiked ? 'Liked' : 'Like'}
            </button>
            <button
              onClick={skipNext}
              className="px-2.5 py-1.5 rounded bg-spotify-elevated text-spotify-text hover:bg-spotify-card-hover transition text-xs"
              title="Skip"
            >
              Skip
            </button>
            <button
              onClick={() => recordFeedback.mutate({ id: nowPlaying.id, feedback: 'ban' })}
              className="px-2.5 py-1.5 rounded bg-red-600 text-white hover:bg-red-700 transition text-xs"
              title="Ban"
            >
              Ban
            </button>
          </div>
        )}
      </div>

      <div className="w-1/4 md:w-1/4 flex flex-col items-end gap-1 text-[10px] md:text-xs text-spotify-subdued">
        <div className="flex items-center gap-1">
          <select
            value={format}
            onChange={(e) => setFormat(e.target.value as StreamFormat)}
            className="bg-spotify-elevated text-spotify-text rounded px-1 py-0.5 border border-spotify-border"
            aria-label="Stream format"
          >
            <option value="flac">FLAC</option>
            <option value="mp3">MP3</option>
            <option value="opus">Opus</option>
            <option value="aac">AAC</option>
          </select>
          {format !== 'flac' && (
            <>
              <select
                value={bitrate}
                onChange={(e) => setBitrate(e.target.value)}
                className="bg-spotify-elevated text-spotify-text rounded px-1 py-0.5 border border-spotify-border w-14"
                aria-label="Bitrate"
              >
                <option value="128">128</option>
                <option value="192">192</option>
                <option value="256">256</option>
                <option value="320">320</option>
              </select>
              <span className="text-spotify-subdued">kbps</span>
            </>
          )}
        </div>
        {nowPlaying && (
          <span className="truncate">{`${nowPlaying.bpm.toFixed(0)} BPM · ${nowPlaying.key}`}</span>
        )}
      </div>
    </div>
  );
}
