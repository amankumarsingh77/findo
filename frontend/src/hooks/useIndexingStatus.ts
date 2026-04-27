import { useSyncExternalStore } from 'react';
import { EventsOn } from '../../wailsjs/runtime/runtime';

export interface FailureGroup {
  code: string;
  label: string;
  count: number;
  sampleFiles: string[];
}

export interface IndexingStatus {
  totalFiles: number;
  indexedFiles: number;
  failedFiles: number;
  currentFile: string;
  isRunning: boolean;
  paused: boolean;
  quotaPaused: boolean;
  quotaResumeAt: string;
  pendingRetryFiles: number;
  failedFileGroups: FailureGroup[];
}

const defaultStatus: IndexingStatus = {
  totalFiles: 0,
  indexedFiles: 0,
  failedFiles: 0,
  currentFile: '',
  isRunning: false,
  paused: false,
  quotaPaused: false,
  quotaResumeAt: '',
  pendingRetryFiles: 0,
  failedFileGroups: [],
};

// Single source of truth for indexing status. Multiple components
// (App's IndexingBar, settings IndexingTab, FoldersTab) all read from this
// one store so they stay in sync. Wails' EventsOff(name) removes all
// listeners for an event, so per-component subscriptions caused listeners
// to clobber each other when components mounted/unmounted.
let current: IndexingStatus = defaultStatus;
const listeners = new Set<() => void>();
let subscribed = false;

function ensureSubscribed() {
  if (subscribed) return;
  subscribed = true;
  EventsOn('indexing-status', (data: IndexingStatus) => {
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

export function useIndexingStatus() {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}
