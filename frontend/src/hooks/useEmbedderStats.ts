import { useSyncExternalStore } from 'react';
import { EventsOn } from '../../wailsjs/runtime/runtime';

export interface EmbedderStats {
  configured: boolean;
  model: string;
  requestsToday: number;
  currentRpm: number;
  maxRpm: number;
  lastEmbedAt: number;
}

const defaultStats: EmbedderStats = {
  configured: false,
  model: '',
  requestsToday: 0,
  currentRpm: 0,
  maxRpm: 0,
  lastEmbedAt: 0,
};

// Singleton store. Same pattern as useIndexingStatus — Wails' EventsOff(name)
// would clobber every listener app-wide, so we keep one subscription here and
// fan out to consumers via useSyncExternalStore.
let current: EmbedderStats = defaultStats;
const listeners = new Set<() => void>();
let subscribed = false;

function ensureSubscribed() {
  if (subscribed) return;
  subscribed = true;
  EventsOn('embedder-stats', (data: EmbedderStats) => {
    current = data;
    listeners.forEach((l) => l());
  });
}

function subscribe(listener: () => void) {
  ensureSubscribed();
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function getSnapshot() {
  return current;
}

// primeEmbedderStats lets a one-shot fetch (e.g. on tab open) seed the store
// without having to wait for the next streaming tick.
export function primeEmbedderStats(data: EmbedderStats) {
  current = data;
  listeners.forEach((l) => l());
}

export function useEmbedderStats() {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}
