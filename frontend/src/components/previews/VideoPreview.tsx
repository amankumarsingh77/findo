import { useState, useEffect } from 'react';
import { GetPreviewClipPath } from '../../../wailsjs/go/main/App';
import type { SearchResultDTO } from '../../hooks/useSearch';

interface VideoPreviewProps {
  result: SearchResultDTO;
}

function formatTimestamp(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m}:${s.toString().padStart(2, '0')}`;
}

export function VideoPreview({ result }: VideoPreviewProps) {
  const [clipPath, setClipPath] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    setClipPath(null);

    GetPreviewClipPath(result.filePath, result.startTime)
      .then((path) => {
        if (!cancelled) {
          setClipPath(path);
          setLoading(false);
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setError(String(err));
          setLoading(false);
        }
      });

    return () => { cancelled = true; };
  }, [result.filePath, result.startTime]);

  return (
    <div style={styles.container}>
      {loading && (
        <div style={styles.loading}>
          <span style={styles.loadingText}>Generating preview clip...</span>
        </div>
      )}
      {error && (
        <div style={styles.loading}>
          <span style={styles.errorText}>Could not generate preview</span>
        </div>
      )}
      {clipPath && !loading && (
        <div style={styles.videoWrap}>
          <video
            key={clipPath}
            src={`wails://localhost/${clipPath}`}
            autoPlay
            muted
            loop
            playsInline
            style={styles.video}
          />
          {result.startTime > 0 && (
            <div style={styles.timestamp}>
              {formatTimestamp(result.startTime)}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    width: '100%',
    borderRadius: 'var(--radius-md)',
    overflow: 'hidden',
    background: '#000',
    aspectRatio: '16/9',
    position: 'relative',
  },
  loading: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: '100%',
    height: '100%',
    position: 'absolute',
    top: 0,
    left: 0,
  },
  loadingText: {
    fontSize: '13px',
    color: 'var(--text-secondary)',
  },
  errorText: {
    fontSize: '13px',
    color: 'var(--accent-warning)',
  },
  videoWrap: {
    width: '100%',
    height: '100%',
    position: 'relative',
  },
  video: {
    width: '100%',
    height: '100%',
    objectFit: 'contain',
  },
  timestamp: {
    position: 'absolute',
    bottom: '8px',
    right: '8px',
    background: 'rgba(0, 0, 0, 0.7)',
    color: 'var(--accent-video)',
    fontSize: '12px',
    fontFamily: 'var(--font-mono)',
    padding: '2px 8px',
    borderRadius: 'var(--radius-sm)',
  },
};
