import { useState, useEffect } from 'react';
import { GetIndexFailures } from '../../wailsjs/go/app/App';
import type { app } from '../../wailsjs/go/models';
import type { FailureGroup } from '../hooks/useIndexingStatus';
import { labelForCode, descriptionForCode } from '../lib/errorLabels';

type IndexFailureDTO = app.IndexFailureDTO;

const MAX_ROWS_PER_GROUP = 200;

function basename(p: string): string {
  const parts = p.replace(/\\/g, '/').split('/');
  return parts[parts.length - 1] || p;
}

interface FailuresModalProps {
  open: boolean;
  onClose: () => void;
  groups: FailureGroup[];
}

export function FailuresModal({ open, onClose, groups }: FailuresModalProps) {
  const [details, setDetails] = useState<IndexFailureDTO[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [expandedCode, setExpandedCode] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setLoading(true);
    setFetchError(null);
    GetIndexFailures()
      .then((result) => {
        setDetails(result);
      })
      .catch((err: unknown) => {
        const msg = err instanceof Error ? err.message : String(err);
        setFetchError(msg);
      })
      .finally(() => {
        setLoading(false);
      });
  }, [open]);

  if (!open) return null;

  const totalFailed = groups.reduce((sum, g) => sum + g.count, 0);

  const detailsForCode = (code: string): IndexFailureDTO[] => {
    if (!details) return [];
    return details.filter((d) => d.code === code);
  };

  return (
    <div
      style={styles.backdrop}
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-label="Indexing failures"
    >
      <div
        style={styles.card}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div style={styles.header}>
          <span style={styles.title}>
            {totalFailed.toLocaleString()} {totalFailed === 1 ? 'file' : 'files'} failed to index
          </span>
          <button style={styles.closeBtn} onClick={onClose} title="Close">×</button>
        </div>

        {/* Body */}
        <div style={styles.body}>
          {loading && (
            <div style={styles.infoRow}>Loading…</div>
          )}
          {!loading && fetchError && (
            <div style={styles.errorRow}>Failed to load details: {fetchError}</div>
          )}
          {!loading && !fetchError && groups.length === 0 && (
            <div style={styles.infoRow}>No failures recorded.</div>
          )}
          {!loading && !fetchError && groups.map((group) => {
            const label = labelForCode(group.code, group.label);
            const description = descriptionForCode(group.code);
            const isExpanded = expandedCode === group.code;
            const rows = detailsForCode(group.code);
            const visibleRows = rows.slice(0, MAX_ROWS_PER_GROUP);
            const overflow = rows.length - visibleRows.length;

            return (
              <div key={group.code} style={styles.group}>
                <button
                  style={styles.groupHeader}
                  onClick={() => setExpandedCode(isExpanded ? null : group.code)}
                  aria-expanded={isExpanded}
                >
                  <span style={styles.groupChevron}>{isExpanded ? '▾' : '▸'}</span>
                  <span style={styles.groupLabel}>{label}</span>
                  <span style={styles.groupCount}>{group.count.toLocaleString()}</span>
                </button>

                {isExpanded && (
                  <div style={styles.groupBody}>
                    {description && (
                      <div style={styles.groupDescription}>{description}</div>
                    )}
                    {details === null ? (
                      <div style={styles.infoRow}>Loading details…</div>
                    ) : visibleRows.length === 0 ? (
                      <div style={styles.infoRow}>No detail records available.</div>
                    ) : (
                      <>
                        {visibleRows.map((row, i) => (
                          <div key={i} style={styles.detailRow} title={row.path}>
                            <span style={styles.detailFilename}>{basename(row.path)}</span>
                            <span style={styles.detailMessage}>{row.message}</span>
                            <span style={styles.detailAttempts}>{row.attempts}x</span>
                          </div>
                        ))}
                        {overflow > 0 && (
                          <div style={styles.overflowNote}>+{overflow} more</div>
                        )}
                      </>
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>

        {/* Footer */}
        <div style={styles.footer}>
          <button style={styles.footerClose} onClick={onClose}>Close</button>
        </div>
      </div>
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  backdrop: {
    position: 'fixed',
    inset: 0,
    background: 'rgba(0,0,0,0.45)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 1000,
  },
  card: {
    background: 'var(--bg-surface)',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius)',
    width: '560px',
    maxWidth: '90vw',
    maxHeight: '70vh',
    display: 'flex',
    flexDirection: 'column',
    boxShadow: '0 8px 32px rgba(0,0,0,0.35)',
    overflow: 'hidden',
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '14px 16px 12px',
    borderBottom: '1px solid var(--border)',
    flexShrink: 0,
  },
  title: {
    fontSize: '14px',
    fontWeight: 600,
    color: 'var(--text-primary)',
  },
  closeBtn: {
    background: 'none',
    border: 'none',
    color: 'var(--text-tertiary)',
    fontSize: '20px',
    cursor: 'pointer',
    padding: '2px 6px',
    borderRadius: 'var(--radius-sm)',
    lineHeight: 1,
  },
  body: {
    flex: 1,
    overflowY: 'auto',
    padding: '8px 0',
  },
  infoRow: {
    fontSize: '12px',
    color: 'var(--text-tertiary)',
    padding: '8px 16px',
  },
  errorRow: {
    fontSize: '12px',
    color: '#e5a00d',
    padding: '8px 16px',
  },
  group: {
    borderBottom: '1px solid var(--border)',
  },
  groupHeader: {
    width: '100%',
    background: 'none',
    border: 'none',
    display: 'flex',
    alignItems: 'center',
    gap: '8px',
    padding: '9px 16px',
    cursor: 'pointer',
    textAlign: 'left',
    color: 'var(--text-primary)',
  },
  groupChevron: {
    fontSize: '10px',
    color: 'var(--text-secondary)',
    width: '12px',
    flexShrink: 0,
  },
  groupLabel: {
    fontSize: '13px',
    fontWeight: 500,
    flex: 1,
  },
  groupCount: {
    fontSize: '12px',
    color: 'var(--text-tertiary)',
    fontFamily: 'var(--font-mono)',
    background: 'var(--bg-surface-2)',
    borderRadius: '10px',
    padding: '1px 7px',
  },
  groupBody: {
    padding: '0 16px 10px 36px',
    display: 'flex',
    flexDirection: 'column',
    gap: '2px',
  },
  groupDescription: {
    fontSize: '11px',
    color: 'var(--text-secondary)',
    marginBottom: '6px',
    lineHeight: 1.5,
  },
  detailRow: {
    display: 'flex',
    alignItems: 'baseline',
    gap: '8px',
    padding: '3px 0',
    borderBottom: '1px solid var(--border)',
  },
  detailFilename: {
    fontSize: '12px',
    color: 'var(--text-primary)',
    fontFamily: 'var(--font-mono)',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
    maxWidth: '220px',
    flexShrink: 0,
  },
  detailMessage: {
    fontSize: '11px',
    color: 'var(--text-secondary)',
    flex: 1,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  detailAttempts: {
    fontSize: '11px',
    color: 'var(--text-tertiary)',
    fontFamily: 'var(--font-mono)',
    flexShrink: 0,
  },
  overflowNote: {
    fontSize: '11px',
    color: 'var(--text-tertiary)',
    padding: '4px 0',
    fontStyle: 'italic',
  },
  footer: {
    display: 'flex',
    justifyContent: 'flex-end',
    padding: '10px 16px',
    borderTop: '1px solid var(--border)',
    flexShrink: 0,
  },
  footerClose: {
    background: 'var(--bg-surface-2)',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-sm)',
    color: 'var(--text-secondary)',
    fontSize: '12px',
    padding: '5px 14px',
    cursor: 'pointer',
    fontFamily: 'var(--font-sans)',
  },
};
