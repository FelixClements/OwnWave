'use client';

import { useEffect, useRef, useState } from 'react';
import { QueueTrack } from '@/server/routers/app';
import { getCoverUrl, getStreamBaseUrl, api } from '@/lib/api';
import { trpc } from '@/lib/trpc/client';

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

export function Player({ queue }: { queue: QueueTrack[] }) {
  const [started, setStarted] = useState(false);
  const [isPlaying, setIsPlaying] = useState(false);
  const [currentTrack, setCurrentTrack] = useState<QueueTrack | null>(null);
  const [coverUrl, setCoverUrl] = useState<string | null>(null);
  const [coverError, setCoverError] = useState(false);
  const [format, setFormat] = useState('mp3');
  const [bitrate, setBitrate] = useState('192');

  const recordPlay = trpc.recordPlay.useMutation();
  const recordFeedback = trpc.recordFeedback.useMutation();

  useEffect(() => {
    const savedFormat = localStorage.getItem('ownwave:format');
    const savedBitrate = localStorage.getItem('ownwave:bitrate');
    if (savedFormat) setFormat(savedFormat);
    if (savedBitrate) setBitrate(savedBitrate);
  }, []);

  useEffect(() => {
    localStorage.setItem('ownwave:format', format);
    localStorage.setItem('ownwave:bitrate', bitrate);
  }, [format, bitrate]);

  const audioARef = useRef<HTMLAudioElement | null>(null);
  const audioBRef = useRef<HTMLAudioElement | null>(null);
  const contextRef = useRef<AudioContext | null>(null);
  const gainARef = useRef<GainNode | null>(null);
  const gainBRef = useRef<GainNode | null>(null);
  const activeRef = useRef<'A' | 'B'>('A');
  const currentIndexRef = useRef(0);
  const crossfadingRef = useRef(false);

  useEffect(() => {
    if (!started || !queue.length) return;

    const AudioCtx =
      (window as any).AudioContext || (window as any).webkitAudioContext;
    if (!AudioCtx) {
      playSequential();
      return;
    }

    if (!contextRef.current) {
      const ctx = new AudioCtx();
      contextRef.current = ctx;

      const master = ctx.createGain();
      master.gain.value = 0.9;
      master.connect(ctx.destination);

      const gainA = ctx.createGain();
      gainA.connect(master);
      gainA.gain.value = 0;
      gainARef.current = gainA;

      const gainB = ctx.createGain();
      gainB.connect(master);
      gainB.gain.value = 0;
      gainBRef.current = gainB;

      if (audioARef.current) {
        const sourceA = ctx.createMediaElementSource(audioARef.current);
        sourceA.connect(gainA);
      }
      if (audioBRef.current) {
        const sourceB = ctx.createMediaElementSource(audioBRef.current);
        sourceB.connect(gainB);
      }
    }

    if (contextRef.current?.state === 'suspended') {
      contextRef.current.resume();
    }

    currentIndexRef.current = 0;
    activeRef.current = 'A';
    loadAndPlay(0, 'A');

    return () => {
      contextRef.current?.close();
      contextRef.current = null;
    };
  }, [started, queue]);

  useEffect(() => {
    if (typeof window === 'undefined' || !('mediaSession' in navigator) || !currentTrack) return;

    const artwork: MediaImage[] = [];
    if (coverUrl && !coverError) {
      artwork.push({
        src: coverUrl,
        sizes: '512x512',
        type: 'image/jpeg',
      });
    }

    navigator.mediaSession.metadata = new MediaMetadata({
      title: currentTrack.title,
      artist: currentTrack.artist || undefined,
      album: currentTrack.album || undefined,
      artwork,
    });

    navigator.mediaSession.setActionHandler('play', () => {
      const audio = activeRef.current === 'A' ? audioARef.current : audioBRef.current;
      if (audio && !isPlaying) audio.play();
      setIsPlaying(true);
    });
    navigator.mediaSession.setActionHandler('pause', () => {
      const audio = activeRef.current === 'A' ? audioARef.current : audioBRef.current;
      if (audio && isPlaying) audio.pause();
      setIsPlaying(false);
    });
    navigator.mediaSession.setActionHandler('nexttrack', () => skipNext());
    navigator.mediaSession.setActionHandler('previoustrack', () => skipPrev());
  }, [currentTrack, coverUrl, coverError, isPlaying]);

  useEffect(() => {
    if (typeof window === 'undefined' || !queue.length) return;
    try {
      localStorage.setItem('ownwave:offlineQueue', JSON.stringify(queue.slice(0, 20)));
    } catch {
      // ignore storage errors
    }
  }, [queue]);

  function playSequential() {
    const audio = audioARef.current;
    if (!audio) return;
    loadTrack(audio, 0).then(() => audio.play());
    audio.addEventListener('ended', () => {
      const next = currentIndexRef.current + 1;
      if (next < queue.length) {
        currentIndexRef.current = next;
        loadTrack(audio, next).then(() => audio.play());
      }
    });
  }

  async function loadTrack(audio: HTMLAudioElement, index: number) {
    const track = queue[index];
    if (!track) return;
    setCurrentTrack(track);
    setCoverUrl(getCoverUrl(track.id));
    setCoverError(false);
    if (track.id) {
      recordPlay.mutate({ id: track.id });
    }
    const { url } = await api.getStreamUrl(track.id, { format: format as any, bitrate });
    audio.src = `${getStreamBaseUrl()}${url}`;
    audio.load();
  }

  async function loadAndPlay(index: number, target: 'A' | 'B') {
    const audio = target === 'A' ? audioARef.current : audioBRef.current;
    const otherAudio = target === 'A' ? audioBRef.current : audioARef.current;
    if (!audio || !otherAudio) return;

    await loadTrack(audio, index);

    otherAudio.pause();
    otherAudio.src = '';

    await audio.play();
    setIsPlaying(true);

    const gain = target === 'A' ? gainARef.current : gainBRef.current;
    const otherGain = target === 'A' ? gainBRef.current : gainARef.current;
    const ctx = contextRef.current;
    if (gain && otherGain && ctx) {
      const now = ctx.currentTime;
      otherGain.gain.cancelScheduledValues(now);
      otherGain.gain.setValueAtTime(0, now);
      gain.gain.cancelScheduledValues(now);
      gain.gain.setValueAtTime(0, now);
      gain.gain.linearRampToValueAtTime(1, now + 0.1);
    }

    activeRef.current = target;
    currentIndexRef.current = index;
    crossfadingRef.current = false;

    audio.ontimeupdate = () => handleTimeUpdate(audio, target);
    audio.onended = () => handleEnded(target);
  }

  function handleTimeUpdate(audio: HTMLAudioElement, target: 'A' | 'B') {
    if (crossfadingRef.current) return;
    const track = queue[currentIndexRef.current];
    if (!track) return;
    if (audio.currentTime >= track.outro_start_seconds - 0.1) {
      const next = currentIndexRef.current + 1;
      if (next < queue.length) {
        beginCrossfade(target, next);
      }
    }
  }

  function handleEnded(target: 'A' | 'B') {
    if (crossfadingRef.current) return;
    const next = currentIndexRef.current + 1;
    if (next < queue.length) {
      loadAndPlay(next, target);
    }
  }

  async function beginCrossfade(currentTarget: 'A' | 'B', nextIndex: number) {
    if (crossfadingRef.current) return;
    crossfadingRef.current = true;

    const nextTarget = currentTarget === 'A' ? 'B' : 'A';
    const currentAudio =
      currentTarget === 'A' ? audioARef.current : audioBRef.current;
    const nextAudio =
      nextTarget === 'A' ? audioARef.current : audioBRef.current;
    const currentGain =
      currentTarget === 'A' ? gainARef.current : gainBRef.current;
    const nextGain =
      nextTarget === 'A' ? gainARef.current : gainBRef.current;
    const ctx = contextRef.current;

    if (!currentAudio || !nextAudio || !currentGain || !nextGain || !ctx) return;

    const nextTrack = queue[nextIndex];
    if (!nextTrack) return;

    const { url } = await api.getStreamUrl(nextTrack.id, { format: format as any, bitrate });
    nextAudio.src = `${getStreamBaseUrl()}${url}`;
    nextAudio.load();
    await nextAudio.play();
    setIsPlaying(true);

    const crossfade = nextTrack.ideal_crossfade_seconds;
    const now = ctx.currentTime;

    currentGain.gain.cancelScheduledValues(now);
    currentGain.gain.setValueAtTime(currentGain.gain.value, now);
    currentGain.gain.linearRampToValueAtTime(0, now + crossfade);

    nextGain.gain.cancelScheduledValues(now);
    nextGain.gain.setValueAtTime(0, now);
    nextGain.gain.linearRampToValueAtTime(1, now + crossfade);

    setTimeout(() => {
      currentAudio.pause();
      currentAudio.src = '';
      currentAudio.ontimeupdate = null;
      currentAudio.onended = null;

      nextAudio.ontimeupdate = () => handleTimeUpdate(nextAudio, nextTarget);
      nextAudio.onended = () => handleEnded(nextTarget);

      activeRef.current = nextTarget;
      currentIndexRef.current = nextIndex;
      crossfadingRef.current = false;
    }, crossfade * 1000);
  }

  function skipNext() {
    if (crossfadingRef.current || !currentTrack) return;
    recordFeedback.mutate({ id: currentTrack.id, feedback: 'skip' });
    const next = currentIndexRef.current + 1;
    if (next < queue.length) {
      const audio = activeRef.current === 'A' ? audioARef.current : audioBRef.current;
      if (audio) {
        audio.ontimeupdate = null;
        audio.onended = null;
      }
      loadAndPlay(next, activeRef.current);
    }
  }

  function skipPrev() {
    if (crossfadingRef.current || !currentTrack) return;
    const prev = currentIndexRef.current - 1;
    if (prev >= 0) {
      const audio = activeRef.current === 'A' ? audioARef.current : audioBRef.current;
      if (audio) {
        audio.ontimeupdate = null;
        audio.onended = null;
      }
      loadAndPlay(prev, activeRef.current);
    }
  }

  function togglePlay() {
    if (!started) {
      setStarted(true);
      return;
    }
    const audio =
      activeRef.current === 'A' ? audioARef.current : audioBRef.current;
    if (!audio) return;
    if (isPlaying) {
      audio.pause();
      setIsPlaying(false);
    } else {
      audio.play().catch(() => {});
      setIsPlaying(true);
    }
  }

  return (
    <div className="w-full h-full flex items-center justify-between gap-2 md:gap-4 px-2 md:px-4">
      <audio ref={audioARef} crossOrigin="anonymous" className="hidden" />
      <audio ref={audioBRef} crossOrigin="anonymous" className="hidden" />

      {currentTrack ? (
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
              currentTrack.title.charAt(0).toUpperCase()
            )}
          </div>
          <div className="min-w-0">
            <div className="text-xs md:text-sm font-bold text-spotify-text truncate">
              {currentTrack.title}
            </div>
            <div className="text-[10px] md:text-xs text-spotify-subdued truncate">
              {currentTrack.artist || 'Unknown artist'}
            </div>
          </div>
        </div>
      ) : (
        <div className="w-5/12 md:w-5/12" />
      )}

      <div className="flex flex-col items-center w-1/3 md:w-1/3 gap-1">
        <button
          onClick={togglePlay}
          className="w-8 h-8 md:w-10 md:h-10 rounded-full bg-spotify-text text-spotify-bg flex items-center justify-center hover:scale-105 transition disabled:opacity-50"
          disabled={started && !currentTrack}
          aria-label={started ? 'Play/Pause' : 'Start radio'}
        >
          {started ? (
            isPlaying ? (
              <PauseIcon className="w-4 h-4 md:w-5 md:h-5" />
            ) : (
              <PlayIcon className="w-4 h-4 md:w-5 md:h-5 ml-0.5" />
            )
          ) : (
            <PlayIcon className="w-4 h-4 md:w-5 md:h-5 ml-0.5" />
          )}
        </button>
        {currentTrack && (
          <div className="flex gap-1 md:gap-2">
            <button
              onClick={() => recordFeedback.mutate({ id: currentTrack.id, feedback: 'like' })}
              className="text-[10px] md:text-xs px-1.5 py-0.5 rounded bg-spotify-elevated text-spotify-text hover:bg-spotify-card-hover transition"
              title="Like"
            >
              Like
            </button>
            <button
              onClick={skipNext}
              className="text-[10px] md:text-xs px-1.5 py-0.5 rounded bg-spotify-elevated text-spotify-text hover:bg-spotify-card-hover transition"
              title="Skip"
            >
              Skip
            </button>
            <button
              onClick={() => recordFeedback.mutate({ id: currentTrack.id, feedback: 'ban' })}
              className="text-[10px] md:text-xs px-1.5 py-0.5 rounded bg-red-600 text-white hover:bg-red-700 transition"
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
            onChange={(e) => setFormat(e.target.value)}
            className="bg-spotify-elevated text-spotify-text rounded px-1 py-0.5 border border-spotify-border"
            aria-label="Stream format"
          >
            <option value="flac">FLAC</option>
            <option value="mp3">MP3</option>
            <option value="opus">Opus</option>
            <option value="aac">AAC</option>
          </select>
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
        </div>
        {currentTrack && (
          <span className="truncate">{`${currentTrack.bpm.toFixed(0)} BPM · ${currentTrack.key}`}</span>
        )}
      </div>
    </div>
  );
}
