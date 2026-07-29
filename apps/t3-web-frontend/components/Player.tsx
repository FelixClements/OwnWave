'use client';

import { useEffect, useRef, useState } from 'react';
import { QueueTrack } from '@/server/routers/app';

const GO_API = process.env.NEXT_PUBLIC_GO_API_URL || 'http://localhost:8080';

async function getStreamUrl(id: string, format = 'mp3') {
  const res = await fetch(`${GO_API}/tracks/${id}/stream-url?format=${format}`);
  const data = await res.json();
  return `${GO_API}${data.url}`;
}

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
    const url = await getStreamUrl(track.id);
    audio.src = url;
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

    const url = await getStreamUrl(nextTrack.id);
    nextAudio.src = url;
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
    <div className="w-full h-full flex items-center justify-between px-4">
      <audio ref={audioARef} crossOrigin="anonymous" className="hidden" />
      <audio ref={audioBRef} crossOrigin="anonymous" className="hidden" />

      {currentTrack ? (
        <div className="flex items-center gap-4 w-1/3 min-w-0">
          <div className="w-14 h-14 bg-spotify-card rounded shadow flex items-center justify-center text-xs text-spotify-subdued font-bold">
            {currentTrack.title.charAt(0).toUpperCase()}
          </div>
          <div className="min-w-0">
            <div className="text-sm font-bold text-spotify-text truncate">
              {currentTrack.title}
            </div>
            <div className="text-xs text-spotify-subdued truncate">
              {currentTrack.artist || 'Unknown artist'}
            </div>
          </div>
        </div>
      ) : (
        <div className="w-1/3" />
      )}

      <div className="flex flex-col items-center w-1/3">
        <button
          onClick={togglePlay}
          className="w-10 h-10 rounded-full bg-spotify-text text-spotify-bg flex items-center justify-center hover:scale-105 transition disabled:opacity-50"
          disabled={started && !currentTrack}
          aria-label={started ? 'Play/Pause' : 'Start radio'}
        >
          {started ? (
            isPlaying ? (
              <PauseIcon className="w-5 h-5" />
            ) : (
              <PlayIcon className="w-5 h-5 ml-0.5" />
            )
          ) : (
            <PlayIcon className="w-5 h-5 ml-0.5" />
          )}
        </button>
      </div>

      <div className="w-1/3 flex justify-end text-xs text-spotify-subdued truncate">
        {currentTrack &&
          `${currentTrack.bpm.toFixed(0)} BPM · ${currentTrack.key}`}
      </div>
    </div>
  );
}
