'use client';

import { useEffect, useRef, useState } from 'react';
import { QueueTrack } from '@/server/routers/app';

const GO_API = process.env.NEXT_PUBLIC_GO_API_URL || 'http://localhost:8080';

async function getStreamUrl(id: string, format = 'mp3') {
  const res = await fetch(`${GO_API}/tracks/${id}/stream-url?format=${format}`);
  const data = await res.json();
  return `${GO_API}${data.url}`;
}

export function Player({ queue }: { queue: QueueTrack[] }) {
  const [started, setStarted] = useState(false);
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

    const AudioCtx = (window as any).AudioContext || (window as any).webkitAudioContext;
    if (!AudioCtx) {
      // Fallback: just play sequential with native audio.
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

    // Reset other audio.
    otherAudio.pause();
    otherAudio.src = '';

    audio.play();

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
    const currentAudio = currentTarget === 'A' ? audioARef.current : audioBRef.current;
    const nextAudio = nextTarget === 'A' ? audioARef.current : audioBRef.current;
    const currentGain = currentTarget === 'A' ? gainARef.current : gainBRef.current;
    const nextGain = nextTarget === 'A' ? gainARef.current : gainBRef.current;
    const ctx = contextRef.current;

    if (!currentAudio || !nextAudio || !currentGain || !nextGain || !ctx) return;

    const nextTrack = queue[nextIndex];
    if (!nextTrack) return;

    const url = await getStreamUrl(nextTrack.id);
    nextAudio.src = url;
    nextAudio.load();
    nextAudio.play();

    const crossfade = nextTrack.ideal_crossfade_seconds;
    const now = ctx.currentTime;

    currentGain.gain.cancelScheduledValues(now);
    currentGain.gain.setValueAtTime(currentGain.gain.value, now);
    currentGain.gain.linearRampToValueAtTime(0, now + crossfade);

    nextGain.gain.cancelScheduledValues(now);
    nextGain.gain.setValueAtTime(0, now);
    nextGain.gain.linearRampToValueAtTime(1, now + crossfade);

    setTimeout(
      () => {
        currentAudio.pause();
        currentAudio.src = '';
        currentAudio.ontimeupdate = null;
        currentAudio.onended = null;

        nextAudio.ontimeupdate = () => handleTimeUpdate(nextAudio, nextTarget);
        nextAudio.onended = () => handleEnded(nextTarget);

        activeRef.current = nextTarget;
        currentIndexRef.current = nextIndex;
        crossfadingRef.current = false;
      },
      crossfade * 1000
    );
  }

  if (!started) {
    return (
      <div className="text-center py-12">
        <button
          onClick={() => setStarted(true)}
          className="bg-blue-600 hover:bg-blue-500 text-white font-semibold py-3 px-8 rounded-full text-lg"
        >
          Start Radio
        </button>
      </div>
    );
  }

  return (
    <div className="bg-zinc-900 p-6 rounded-xl">
      <audio ref={audioARef} crossOrigin="anonymous" className="hidden" />
      <audio ref={audioBRef} crossOrigin="anonymous" className="hidden" />

      {currentTrack && (
        <div className="mb-4">
          <h3 className="text-2xl font-semibold">{currentTrack.title}</h3>
          <p className="text-zinc-400">
            {currentTrack.artist} · {currentTrack.bpm.toFixed(0)} BPM ·{' '}
            {currentTrack.key}
          </p>
        </div>
      )}

      <div className="flex gap-4 justify-center">
        <button
          onClick={() => audioARef.current?.pause()}
          className="bg-zinc-700 hover:bg-zinc-600 px-4 py-2 rounded"
        >
          Pause
        </button>
      </div>
    </div>
  );
}
