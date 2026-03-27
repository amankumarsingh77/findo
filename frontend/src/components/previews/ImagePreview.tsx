import type { SearchResultDTO } from '../../hooks/useSearch';

interface ImagePreviewProps {
  result: SearchResultDTO;
}

export function ImagePreview({ result }: ImagePreviewProps) {
  return (
    <div style={styles.container}>
      <img
        src={`wails://localhost/${result.filePath}`}
        alt={result.fileName}
        style={styles.image}
        onError={(e) => {
          (e.target as HTMLImageElement).style.display = 'none';
        }}
      />
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    width: '100%',
    borderRadius: 'var(--radius-md)',
    overflow: 'hidden',
    background: '#000',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    maxHeight: '400px',
  },
  image: {
    maxWidth: '100%',
    maxHeight: '400px',
    objectFit: 'contain',
  },
};
