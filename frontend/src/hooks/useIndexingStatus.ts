import { useState, useEffect } from 'react';
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';

export interface IndexingStatus {
  totalFiles: number;
  indexedFiles: number;
  currentFile: string;
  isRunning: boolean;
}

const defaultStatus: IndexingStatus = {
  totalFiles: 0,
  indexedFiles: 0,
  currentFile: '',
  isRunning: false,
};

export function useIndexingStatus() {
  const [status, setStatus] = useState<IndexingStatus>(defaultStatus);

  useEffect(() => {
    EventsOn('indexing-status', (data: IndexingStatus) => {
      setStatus(data);
    });

    return () => {
      EventsOff('indexing-status');
    };
  }, []);

  return status;
}
