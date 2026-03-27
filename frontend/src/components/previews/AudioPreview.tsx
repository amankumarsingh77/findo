import type { SearchResultDTO } from '../../hooks/useSearch';

interface AudioPreviewProps {
  result: SearchResultDTO;
}

export function AudioPreview({ result }: AudioPreviewProps) {
  return (
    <div style={styles.container}>
      <div style={styles.visualizer}>
        <div style={styles.icon}>🎵</div>
        <div style={styles.waveform}>
          {Array.from({ length: 32 }).map((_, i) => (
            <div
              key={i}
              style={{
                ...styles.bar,
                height: `${20 + Math.sin(i * 0.5) * 15 + Math.random() * 10}px`,
              }}
            />
          ))}
        </div>
      </div>
      <audio
        src={`wails://localhost/${result.filePath}`}
        controls
        style={styles.audio}
      />
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    width: '100%',
    borderRadius: 'var(--radius-md)',
    overflow: 'hidden',
    background: 'var(--bg-surface-2)',
    border: '1px solid var(--border)',
  },
  visualizer: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    padding: '32px 16px 16px',
    gap: '16px',
  },
  icon: {
    fontSize: '36px',
  },
  waveform: {
    display: 'flex',
    alignItems: 'flex-end',
    gap: '2px',
    height: '48px',
  },
  bar: {
    width: '4px',
    background: 'var(--accent-audio)',
    borderRadius: '2px',
    opacity: 0.6,
  },
  audio: {
    width: '100%',
    display: 'block',
    outline: 'none',
  },
};
