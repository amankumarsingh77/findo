import type { SearchResultDTO } from '../../hooks/useSearch';

interface TextPreviewProps {
  result: SearchResultDTO;
}

export function TextPreview({ result }: TextPreviewProps) {
  const isCode = result.fileType === 'code';

  return (
    <div style={styles.container}>
      <div
        style={{
          ...styles.content,
          borderColor: isCode ? 'var(--accent-code)' : 'var(--border)',
        }}
      >
        <div style={styles.header}>
          <span style={{ ...styles.badge, background: isCode ? 'var(--accent-code)' : 'var(--text-secondary)' }}>
            {isCode ? 'CODE' : 'TEXT'}
          </span>
          <span style={styles.ext}>{result.extension}</span>
        </div>
        <div style={styles.body}>
          <span style={styles.placeholder}>
            Content preview available when file is opened
          </span>
        </div>
      </div>
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    width: '100%',
  },
  content: {
    borderRadius: 'var(--radius-md)',
    border: '1px solid',
    overflow: 'hidden',
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    gap: '8px',
    padding: '8px 12px',
    borderBottom: '1px solid var(--border)',
    background: 'var(--bg-surface)',
  },
  badge: {
    fontSize: '10px',
    fontFamily: 'var(--font-mono)',
    fontWeight: 600,
    color: '#000',
    padding: '1px 6px',
    borderRadius: 'var(--radius-sm)',
    textTransform: 'uppercase' as const,
  },
  ext: {
    fontSize: '12px',
    color: 'var(--text-secondary)',
    fontFamily: 'var(--font-mono)',
  },
  body: {
    padding: '16px',
    background: 'var(--bg-surface-2)',
    minHeight: '120px',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
  },
  placeholder: {
    fontSize: '13px',
    color: 'var(--text-tertiary)',
    fontFamily: 'var(--font-mono)',
  },
};
