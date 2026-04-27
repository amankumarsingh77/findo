import { useState, useEffect } from 'react';
import { KeyRound, Eye, EyeOff, Activity, TrendingUp } from 'lucide-react';
import { SetGeminiAPIKey, GetHasGeminiKey, GetEmbedderStats } from '../../../wailsjs/go/app/App';
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
import { useEmbedderStats, primeEmbedderStats } from '../../hooks/useEmbedderStats';

interface ApiKeyTabProps {
  onSuccess?: () => void;
  onError?: (msg: string) => void;
}

export function ApiKeyTab({ onSuccess, onError }: ApiKeyTabProps) {
  const [key, setKey] = useState('');
  const [showKey, setShowKey] = useState(false);
  const [loading, setLoading] = useState(false);
  const [hasKey, setHasKey] = useState(false);
  const [errorMsg, setErrorMsg] = useState('');

  const stats = useEmbedderStats();

  useEffect(() => {
    GetHasGeminiKey()
      .then(setHasKey)
      .catch(() => setHasKey(false));
    // Prime the store so the first render isn't blank — the streaming event
    // arrives every second but we don't want to wait for the first tick.
    GetEmbedderStats().then(primeEmbedderStats).catch(() => {});
  }, []);

  const handleSave = async () => {
    if (key.trim() === '') {
      setErrorMsg('API key must not be empty');
      onError?.('API key must not be empty');
      return;
    }
    setLoading(true);
    setErrorMsg('');
    try {
      await SetGeminiAPIKey(key);
      setHasKey(true);
      setKey('');
      onSuccess?.();
    } catch (err) {
      setErrorMsg(String(err));
      onError?.(String(err));
    } finally {
      setLoading(false);
    }
  };

  const connected = hasKey && stats.configured;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      <div>
        <h2 style={styles.heading}>Gemini API Key</h2>
        <p style={styles.subtext}>
          Findo uses Gemini Embedding 2 to power semantic search. Your key is stored locally in the system keychain — never sent anywhere except Google.
        </p>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        <label style={styles.label}>API key</label>
        <div style={styles.inputWrapper}>
          <KeyRound size={16} color="var(--text-tertiary)" style={{ flexShrink: 0 }} />
          <input
            type={showKey ? 'text' : 'password'}
            value={key}
            onChange={(e) => setKey(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleSave()}
            placeholder={hasKey ? '••••••••••••••••••••••' : 'Enter Gemini API key'}
            style={styles.input}
            autoComplete="off"
            spellCheck={false}
          />
          <button
            onClick={() => setShowKey(!showKey)}
            style={styles.eyeBtn}
            title={showKey ? 'Hide key' : 'Show key'}
          >
            {showKey ? <EyeOff size={16} /> : <Eye size={16} />}
          </button>
        </div>
      </div>

      {connected ? (
        <StatusCard stats={stats} />
      ) : errorMsg ? (
        <div style={styles.errorCard}>
          <div style={{ fontWeight: 600, color: '#e53935', fontSize: 14 }}>Invalid key</div>
          <div style={{ fontSize: 13, color: 'var(--text-secondary)', marginTop: 2 }}>{errorMsg}</div>
        </div>
      ) : null}

      <div style={{ display: 'flex', gap: 10 }}>
        <button style={styles.primaryBtn} onClick={handleSave} disabled={loading}>
          {loading ? 'Verifying…' : 'Save & Verify'}
        </button>
        <button
          style={styles.secondaryBtn}
          onClick={() => BrowserOpenURL('https://aistudio.google.com/apikey')}
        >
          Get a key
        </button>
      </div>
    </div>
  );
}

interface StatusCardProps {
  stats: ReturnType<typeof useEmbedderStats>;
}

function StatusCard({ stats }: StatusCardProps) {
  return (
    <div style={styles.statusCard}>
      <div style={styles.statusRow}>
        <div style={styles.statusLeft}>
          <span style={styles.statusDot} />
          <span style={styles.statusLabel}>Connected</span>
          {stats.model && (
            <>
              <span style={styles.sep}>·</span>
              <span style={styles.statusModel}>{stats.model}</span>
            </>
          )}
        </div>
        <span style={styles.statusVerified}>{relativeTime(stats.lastEmbedAt) ?? 'verified just now'}</span>
      </div>
      <div style={styles.statusDivider} />
      <div style={styles.kpiRow}>
        <div style={styles.kpi}>
          <Activity size={12} color="var(--text-secondary)" />
          <span style={styles.kpiText}>
            {stats.requestsToday.toLocaleString()} reqs today
          </span>
        </div>
        <div style={styles.kpi}>
          <TrendingUp size={12} color="var(--text-secondary)" />
          <span style={styles.kpiText}>
            {stats.currentRpm} / {stats.maxRpm || '—'} req·min
          </span>
        </div>
      </div>
    </div>
  );
}

// relativeTime formats a unix-seconds timestamp as a short relative string.
// Returns null when the timestamp is the zero/never value.
function relativeTime(unix: number): string | null {
  if (!unix) return null;
  const diff = Math.max(0, Math.floor(Date.now() / 1000) - unix);
  if (diff < 5) return 'last call just now';
  if (diff < 60) return `last call ${diff}s ago`;
  if (diff < 3600) return `last call ${Math.floor(diff / 60)}m ago`;
  return `last call ${Math.floor(diff / 3600)}h ago`;
}

const styles: Record<string, React.CSSProperties> = {
  heading: {
    fontSize: 22, fontWeight: 700, color: 'var(--text-primary)',
    margin: '0 0 8px', fontFamily: 'var(--font-sans)',
  },
  subtext: {
    fontSize: 14, color: 'var(--text-secondary)',
    margin: 0, lineHeight: 1.6, fontFamily: 'var(--font-sans)',
  },
  label: {
    fontSize: 13, color: 'var(--text-secondary)',
    fontWeight: 500, fontFamily: 'var(--font-sans)',
  },
  inputWrapper: {
    display: 'flex', alignItems: 'center', gap: 10,
    background: 'var(--bg-surface-2, var(--bg-surface))',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-lg, 12px)',
    padding: '10px 14px',
  },
  input: {
    flex: 1, background: 'transparent', border: 'none', outline: 'none',
    color: 'var(--text-primary)', fontSize: 14, fontFamily: 'var(--font-mono)',
  },
  eyeBtn: {
    background: 'none', border: 'none', color: 'var(--text-tertiary)',
    cursor: 'pointer', display: 'flex', alignItems: 'center',
    padding: 0, flexShrink: 0,
  },
  statusCard: {
    background: 'var(--bg-surface-2, var(--bg-surface))',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-lg, 12px)',
    padding: '14px 16px',
    display: 'flex', flexDirection: 'column', gap: 12,
  },
  statusRow: {
    display: 'flex', alignItems: 'center', justifyContent: 'space-between',
    gap: 10, flexWrap: 'wrap' as const,
  },
  statusLeft: {
    display: 'flex', alignItems: 'center', gap: 8, minWidth: 0,
  },
  statusDot: {
    width: 8, height: 8, borderRadius: '50%',
    background: 'var(--accent-green)',
    display: 'inline-block', flexShrink: 0,
  },
  statusLabel: {
    fontSize: 14, fontWeight: 600, color: 'var(--text-primary)',
    fontFamily: 'var(--font-sans)',
  },
  sep: {
    color: 'var(--text-tertiary)', fontSize: 14,
  },
  statusModel: {
    fontSize: 12, color: 'var(--text-secondary)',
    fontFamily: 'var(--font-mono)',
    overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const,
  },
  statusVerified: {
    fontSize: 11, color: 'var(--text-tertiary)',
    fontFamily: 'var(--font-sans)', flexShrink: 0,
  },
  statusDivider: {
    height: 1, background: 'var(--border)', width: '100%',
  },
  kpiRow: {
    display: 'flex', alignItems: 'center', gap: 18,
    flexWrap: 'wrap' as const,
  },
  kpi: {
    display: 'inline-flex', alignItems: 'center', gap: 6,
  },
  kpiText: {
    fontSize: 12, color: 'var(--text-primary)',
    fontFamily: 'var(--font-sans)',
  },
  errorCard: {
    background: 'rgba(229,57,53,0.08)',
    border: '1px solid rgba(229,57,53,0.2)',
    borderRadius: 'var(--radius-lg, 12px)',
    padding: '12px 16px',
  },
  primaryBtn: {
    background: 'var(--accent, #7c6fe0)', border: 'none',
    borderRadius: 'var(--radius-lg, 12px)', color: '#fff',
    fontSize: 14, fontWeight: 600, padding: '10px 20px',
    cursor: 'pointer', fontFamily: 'var(--font-sans)',
  },
  secondaryBtn: {
    background: 'transparent', border: '1px solid var(--border)',
    borderRadius: 'var(--radius-lg, 12px)', color: 'var(--text-secondary)',
    fontSize: 14, fontWeight: 500, padding: '10px 20px',
    cursor: 'pointer', fontFamily: 'var(--font-sans)',
  },
};
