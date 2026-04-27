import {
  useState,
  useEffect,
  useCallback,
  useRef,
  useLayoutEffect,
  useMemo,
  useDeferredValue,
  memo,
} from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { Folder, MoreHorizontal, RotateCcw, X, Plus, Search, FolderPlus } from 'lucide-react';
import {
  GetFolders, RemoveFolder, PickAndAddFolder, ReindexFolder,
  GetIgnoredFolders, AddIgnoredFolder, RemoveIgnoredFolder,
} from '../../../wailsjs/go/app/App';
import { EventsOn } from '../../../wailsjs/runtime/runtime';
import { useHideSuppression } from '../../hooks/useHideSuppression';
import { useIndexingStatus } from '../../hooks/useIndexingStatus';

type ConfirmState = { path: string } | null;
type SubTab = 'folders' | 'excluded';

const FOLDER_ROW_HEIGHT = 64;
const PATTERN_ROW_HEIGHT = 48;
const LIST_HEIGHT = 420;

const shortenPath = (p: string) => p.replace(/^\/Users\/[^/]+/, '~');

export function FoldersTab() {
  const [tab, setTab] = useState<SubTab>('folders');

  const [folders, setFolders] = useState<string[]>([]);
  const [ignoredPatterns, setIgnoredPatterns] = useState<string[]>([]);
  const [folderQuery, setFolderQuery] = useState('');
  const [patternQuery, setPatternQuery] = useState('');
  const [newPattern, setNewPattern] = useState('');
  const [confirm, setConfirm] = useState<ConfirmState>(null);
  const [reindexingFolder, setReindexingFolder] = useState<string | null>(null);
  const [pickingFolder, setPickingFolder] = useState(false);
  const [openMenu, setOpenMenu] = useState<string | null>(null);
  const [menuPos, setMenuPos] = useState<{ top: number; left: number } | null>(null);
  const [addingPattern, setAddingPattern] = useState(false);
  const [patternFocused, setPatternFocused] = useState(false);
  const menuBtnRefs = useRef<Record<string, HTMLButtonElement | null>>({});
  const patternInputRef = useRef<HTMLInputElement>(null);
  const folderListRef = useRef<HTMLDivElement>(null);
  const patternListRef = useRef<HTMLDivElement>(null);
  const { withSuppressedHide } = useHideSuppression();
  const indexingStatus = useIndexingStatus();

  // Defer the search input so typing remains responsive even with thousands
  // of folders — filtering happens at a slightly lower priority.
  const deferredFolderQuery = useDeferredValue(folderQuery);
  const deferredPatternQuery = useDeferredValue(patternQuery);

  const loadFolders = useCallback(async () => {
    try {
      const result = await GetFolders();
      setFolders(result || []);
    } catch (err) {
      console.error('Failed to load folders:', err);
    }
  }, []);

  const loadIgnoredPatterns = useCallback(async () => {
    try {
      const result = await GetIgnoredFolders();
      setIgnoredPatterns(result || []);
    } catch (err) {
      console.error('Failed to load ignored patterns:', err);
    }
  }, []);

  useEffect(() => {
    loadFolders();
    loadIgnoredPatterns();
  }, [loadFolders, loadIgnoredPatterns]);

  // EventsOn returns a per-listener cancel function. Using EventsOff(name)
  // would tear down every listener for the event app-wide.
  useEffect(() => {
    const cancel = EventsOn('folders-changed', () => loadFolders());
    return () => {
      if (typeof cancel === 'function') cancel();
    };
  }, [loadFolders]);

  // Filtered folder list. Memoized on the deferred query so typing stays
  // smooth — the filter pass runs once per query, not per keystroke render.
  const filteredFolders = useMemo(() => {
    const q = deferredFolderQuery.trim().toLowerCase();
    if (!q) return folders;
    return folders.filter((f) => f.toLowerCase().includes(q));
  }, [folders, deferredFolderQuery]);

  const filteredPatterns = useMemo(() => {
    const q = deferredPatternQuery.trim().toLowerCase();
    if (!q) return ignoredPatterns;
    return ignoredPatterns.filter((p) => p.toLowerCase().includes(q));
  }, [ignoredPatterns, deferredPatternQuery]);

  // Avoid O(N) prefix checks per row. The indexing pipeline only ever
  // reports one currentFile at a time, so at most one folder is "Indexing"
  // per status tick — compute that folder once.
  const indexingFolder = useMemo<string | null>(() => {
    if (!indexingStatus.isRunning) return null;
    const cur = indexingStatus.currentFile;
    if (!cur) return null;
    for (const f of folders) {
      const fSlash = f.endsWith('/') ? f : f + '/';
      if (cur === f || cur.startsWith(fSlash)) return f;
    }
    return null;
  }, [folders, indexingStatus.isRunning, indexingStatus.currentFile]);

  const folderVirtualizer = useVirtualizer({
    count: filteredFolders.length,
    getScrollElement: () => folderListRef.current,
    estimateSize: () => FOLDER_ROW_HEIGHT,
    overscan: 6,
  });

  const patternVirtualizer = useVirtualizer({
    count: filteredPatterns.length,
    getScrollElement: () => patternListRef.current,
    estimateSize: () => PATTERN_ROW_HEIGHT,
    overscan: 6,
  });

  // Position the floating row-menu next to its trigger button.
  useLayoutEffect(() => {
    if (!openMenu) {
      setMenuPos(null);
      return;
    }
    const btn = menuBtnRefs.current[openMenu];
    if (!btn) return;
    const rect = btn.getBoundingClientRect();
    const menuWidth = 150;
    const menuHeight = 78;
    const fitsBelow = rect.bottom + menuHeight + 8 < window.innerHeight;
    setMenuPos({
      top: fitsBelow ? rect.bottom + 4 : rect.top - menuHeight - 4,
      left: Math.max(8, rect.right - menuWidth),
    });
  }, [openMenu]);

  useEffect(() => {
    if (!openMenu) return;
    const onDown = (e: MouseEvent) => {
      const btn = menuBtnRefs.current[openMenu];
      if (btn && btn.contains(e.target as Node)) return;
      const target = e.target as HTMLElement | null;
      if (target?.closest('[data-folder-menu="true"]')) return;
      setOpenMenu(null);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpenMenu(null);
    };
    document.addEventListener('mousedown', onDown);
    window.addEventListener('keydown', onKey, true);
    return () => {
      document.removeEventListener('mousedown', onDown);
      window.removeEventListener('keydown', onKey, true);
    };
  }, [openMenu]);

  useEffect(() => {
    if (addingPattern) patternInputRef.current?.focus();
  }, [addingPattern]);

  const handleAddFolder = async () => {
    if (pickingFolder) return;
    setPickingFolder(true);
    try {
      await withSuppressedHide(() => PickAndAddFolder());
      await loadFolders();
    } catch (err) {
      console.error('Failed to add folder:', err);
    } finally {
      setPickingFolder(false);
    }
  };

  const handleRemove = async (path: string, deleteData: boolean) => {
    try {
      await RemoveFolder(path, deleteData);
      setConfirm(null);
      await loadFolders();
    } catch (err) {
      console.error('Failed to remove folder:', err);
    }
  };

  const handleReindex = async (path: string) => {
    if (reindexingFolder) return;
    setReindexingFolder(path);
    setOpenMenu(null);
    try {
      await ReindexFolder(path);
    } catch (err) {
      console.error('Failed to reindex folder:', err);
    } finally {
      setTimeout(() => setReindexingFolder(null), 1500);
    }
  };

  const handleAddPattern = async () => {
    const trimmed = newPattern.trim();
    if (!trimmed) {
      setAddingPattern(false);
      return;
    }
    try {
      await AddIgnoredFolder(trimmed);
      setNewPattern('');
      setAddingPattern(false);
      await loadIgnoredPatterns();
    } catch (err) {
      console.error('Failed to add pattern:', err);
    }
  };

  const handleRemovePattern = async (pattern: string) => {
    try {
      await RemoveIgnoredFolder(pattern);
      await loadIgnoredPatterns();
    } catch (err) {
      console.error('Failed to remove pattern:', err);
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <h2 style={styles.heading}>Folders</h2>
      <p style={styles.subtext}>What Findo scans and what it skips.</p>

      {/* Sub-tab bar */}
      <div style={styles.tabBar}>
        <TabButton
          active={tab === 'folders'}
          label="Folders"
          count={folders.length}
          onClick={() => setTab('folders')}
        />
        <TabButton
          active={tab === 'excluded'}
          label="Excluded"
          count={ignoredPatterns.length}
          onClick={() => setTab('excluded')}
        />
      </div>

      {tab === 'folders' && (
        <>
          <div style={styles.toolbar}>
            <div style={styles.searchWrap}>
              <Search size={14} color="var(--text-tertiary)" style={{ flexShrink: 0 }} />
              <input
                style={styles.searchInput}
                type="text"
                placeholder="Filter folders…"
                value={folderQuery}
                onChange={(e) => setFolderQuery(e.target.value)}
              />
            </div>
            <button
              style={styles.addBtn}
              onClick={handleAddFolder}
              disabled={pickingFolder}
            >
              <FolderPlus size={14} />
              {pickingFolder ? 'Opening…' : 'Add Folder'}
            </button>
          </div>

          {filteredFolders.length === 0 ? (
            <div style={styles.empty}>
              {folders.length === 0
                ? 'No folders indexed yet. Add a folder to get started.'
                : 'No folders match your filter.'}
            </div>
          ) : (
            <div ref={folderListRef} style={{ ...styles.virtualList, height: LIST_HEIGHT }}>
              <div
                style={{
                  height: folderVirtualizer.getTotalSize(),
                  width: '100%',
                  position: 'relative',
                }}
              >
                {folderVirtualizer.getVirtualItems().map((vRow) => {
                  const folder = filteredFolders[vRow.index];
                  const indexing = reindexingFolder === folder || indexingFolder === folder;
                  return (
                    <div
                      key={folder}
                      style={{
                        position: 'absolute',
                        top: 0,
                        left: 0,
                        width: '100%',
                        height: vRow.size,
                        transform: `translateY(${vRow.start}px)`,
                      }}
                    >
                      <FolderRow
                        folder={folder}
                        indexing={indexing}
                        reindexLabel={reindexingFolder === folder}
                        menuBtnRef={(el) => { menuBtnRefs.current[folder] = el; }}
                        onMenuToggle={() =>
                          setOpenMenu(openMenu === folder ? null : folder)
                        }
                      />
                    </div>
                  );
                })}
              </div>
            </div>
          )}

          {confirm && (
            <div style={styles.confirmOverlay}>
              <div style={styles.confirmText}>
                Remove <strong>{confirm.path.split('/').pop()}</strong>?
              </div>
              <div style={styles.confirmBtns}>
                <button style={styles.dangerBtn} onClick={() => handleRemove(confirm.path, true)}>
                  Remove &amp; Delete Data
                </button>
                <button style={styles.secondaryBtn} onClick={() => handleRemove(confirm.path, false)}>
                  Keep Data
                </button>
                <button style={styles.secondaryBtn} onClick={() => setConfirm(null)}>
                  Cancel
                </button>
              </div>
            </div>
          )}
        </>
      )}

      {tab === 'excluded' && (
        <>
          <p style={styles.tabHint}>Files matching any pattern are skipped across all folders.</p>
          <div style={styles.toolbar}>
            <div style={styles.searchWrap}>
              <Search size={14} color="var(--text-tertiary)" style={{ flexShrink: 0 }} />
              <input
                style={styles.searchInput}
                type="text"
                placeholder="Filter patterns…"
                value={patternQuery}
                onChange={(e) => setPatternQuery(e.target.value)}
              />
            </div>
            {addingPattern ? (
              <input
                ref={patternInputRef}
                style={{
                  ...styles.patternInput,
                  borderColor: patternFocused ? 'var(--border-focus)' : 'var(--border)',
                }}
                type="text"
                placeholder="e.g. node_modules"
                value={newPattern}
                onChange={(e) => setNewPattern(e.target.value)}
                onFocus={() => setPatternFocused(true)}
                onBlur={() => {
                  setPatternFocused(false);
                  if (!newPattern.trim()) setAddingPattern(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault();
                    handleAddPattern();
                  } else if (e.key === 'Escape') {
                    e.preventDefault();
                    setNewPattern('');
                    setAddingPattern(false);
                  }
                }}
              />
            ) : (
              <button style={styles.addBtn} onClick={() => setAddingPattern(true)}>
                <Plus size={14} /> Add pattern
              </button>
            )}
          </div>

          {filteredPatterns.length === 0 ? (
            <div style={styles.empty}>
              {ignoredPatterns.length === 0
                ? 'No excluded patterns yet.'
                : 'No patterns match your filter.'}
            </div>
          ) : (
            <div ref={patternListRef} style={{ ...styles.virtualList, height: LIST_HEIGHT }}>
              <div
                style={{
                  height: patternVirtualizer.getTotalSize(),
                  width: '100%',
                  position: 'relative',
                }}
              >
                {patternVirtualizer.getVirtualItems().map((vRow) => {
                  const pattern = filteredPatterns[vRow.index];
                  return (
                    <div
                      key={pattern}
                      style={{
                        position: 'absolute',
                        top: 0,
                        left: 0,
                        width: '100%',
                        height: vRow.size,
                        transform: `translateY(${vRow.start}px)`,
                      }}
                    >
                      <PatternRow pattern={pattern} onRemove={handleRemovePattern} />
                    </div>
                  );
                })}
              </div>
            </div>
          )}
        </>
      )}

      {openMenu && menuPos && (
        <div
          data-folder-menu="true"
          style={{
            ...styles.dropMenu,
            position: 'fixed',
            top: menuPos.top,
            left: menuPos.left,
          }}
        >
          <button style={styles.dropItem} onClick={() => handleReindex(openMenu)}>
            <RotateCcw size={13} style={{ marginRight: 6 }} /> Rescan
          </button>
          <button
            style={{ ...styles.dropItem, color: '#e53935' }}
            onClick={() => { setConfirm({ path: openMenu }); setOpenMenu(null); }}
          >
            <X size={13} style={{ marginRight: 6 }} /> Remove
          </button>
        </div>
      )}
    </div>
  );
}

interface TabButtonProps {
  active: boolean;
  label: string;
  count: number;
  onClick: () => void;
}

function TabButton({ active, label, count, onClick }: TabButtonProps) {
  return (
    <button
      style={{
        ...styles.tabBtn,
        color: active ? 'var(--text-primary)' : 'var(--text-secondary)',
        borderBottom: active ? '2px solid var(--accent, #7c6fe0)' : '2px solid transparent',
        fontWeight: active ? 600 : 500,
      }}
      onClick={onClick}
    >
      {label}
      <span
        style={{
          ...styles.tabBadge,
          background: active ? 'var(--bg-selected)' : 'rgba(255,255,255,0.07)',
          color: active ? 'var(--text-primary)' : 'var(--text-secondary)',
        }}
      >
        {count}
      </span>
    </button>
  );
}

interface FolderRowProps {
  folder: string;
  indexing: boolean;
  reindexLabel: boolean;
  menuBtnRef: (el: HTMLButtonElement | null) => void;
  onMenuToggle: () => void;
}

const FolderRow = memo(function FolderRow({
  folder, indexing, reindexLabel, menuBtnRef, onMenuToggle,
}: FolderRowProps) {
  return (
    <div style={styles.folderRow}>
      <Folder size={18} color="var(--text-tertiary)" style={{ flexShrink: 0, marginRight: 12 }} />
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={styles.folderPath}>{shortenPath(folder)}</div>
        <div style={styles.folderMeta}>
          {indexing ? (reindexLabel ? 'Reindexing…' : 'Scanning…') : 'last scanned recently'}
        </div>
      </div>
      <span
        style={{
          ...styles.statusPill,
          background: indexing ? 'rgba(124,111,224,0.15)' : 'rgba(16,185,129,0.15)',
          color: indexing ? 'var(--accent, #7c6fe0)' : 'var(--accent-green)',
        }}
      >
        <span
          style={{
            width: 6, height: 6, borderRadius: '50%',
            background: indexing ? 'var(--accent, #7c6fe0)' : 'var(--accent-green)',
            display: 'inline-block', marginRight: 5,
          }}
        />
        {indexing ? 'Indexing' : 'Up to date'}
      </span>
      <button ref={menuBtnRef} style={styles.iconBtn} onClick={onMenuToggle} title="Options">
        <MoreHorizontal size={16} />
      </button>
    </div>
  );
});

interface PatternRowProps {
  pattern: string;
  onRemove: (p: string) => void;
}

const PatternRow = memo(function PatternRow({ pattern, onRemove }: PatternRowProps) {
  return (
    <div style={styles.patternRow}>
      <span style={styles.patternText}>{pattern}</span>
      <button style={styles.iconBtn} onClick={() => onRemove(pattern)} title="Remove">
        <X size={14} />
      </button>
    </div>
  );
});

const styles: Record<string, React.CSSProperties> = {
  heading: {
    fontSize: 22, fontWeight: 700, color: 'var(--text-primary)',
    margin: '0 0 4px', fontFamily: 'var(--font-sans)',
  },
  subtext: {
    fontSize: 14, color: 'var(--text-secondary)',
    margin: '0 0 16px', lineHeight: 1.5, fontFamily: 'var(--font-sans)',
  },
  tabHint: {
    fontSize: 12, color: 'var(--text-tertiary)',
    margin: '0 0 12px', fontFamily: 'var(--font-sans)',
  },
  tabBar: {
    display: 'flex', gap: 4, borderBottom: '1px solid var(--border)',
    marginBottom: 16,
  },
  tabBtn: {
    display: 'inline-flex', alignItems: 'center', gap: 8,
    background: 'transparent', border: 'none',
    padding: '8px 14px', cursor: 'pointer',
    fontSize: 13, fontFamily: 'var(--font-sans)',
    marginBottom: -1,
  },
  tabBadge: {
    display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
    minWidth: 20, height: 18, padding: '0 6px',
    borderRadius: 100, fontSize: 11, fontWeight: 600,
  },
  toolbar: {
    display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12,
  },
  searchWrap: {
    flex: 1, display: 'flex', alignItems: 'center', gap: 8,
    height: 34, padding: '0 12px',
    background: 'var(--bg-surface-opaque-2)',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-md, 8px)',
  },
  searchInput: {
    flex: 1, background: 'transparent', border: 'none', outline: 'none',
    color: 'var(--text-primary)', fontSize: 13,
    fontFamily: 'var(--font-sans)',
  },
  addBtn: {
    display: 'flex', alignItems: 'center', gap: 6,
    background: 'var(--accent, #7c6fe0)', border: 'none',
    borderRadius: 'var(--radius-md, 8px)',
    color: '#fff', fontSize: 13, fontWeight: 600,
    padding: '8px 14px', cursor: 'pointer',
    fontFamily: 'var(--font-sans)', flexShrink: 0,
  },
  patternInput: {
    background: 'var(--bg-surface-opaque-2)',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-md, 8px)',
    color: 'var(--text-primary)', fontSize: 13,
    padding: '7px 12px', fontFamily: 'var(--font-mono)',
    outline: 'none', width: 200,
  },
  empty: {
    padding: '24px 0', fontSize: 14,
    color: 'var(--text-tertiary)', textAlign: 'center',
  },
  virtualList: {
    width: '100%',
    overflowY: 'auto',
    background: 'var(--bg-surface)',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-lg, 12px)',
    contain: 'strict',
  },
  folderRow: {
    display: 'flex', alignItems: 'center', height: '100%',
    padding: '0 16px', gap: 8,
    borderBottom: '1px solid var(--border)',
  },
  folderPath: {
    fontSize: 14, color: 'var(--text-primary)',
    fontFamily: 'var(--font-mono)',
    overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const,
  },
  folderMeta: {
    fontSize: 12, color: 'var(--text-tertiary)', marginTop: 2,
  },
  statusPill: {
    display: 'inline-flex', alignItems: 'center',
    padding: '3px 10px', borderRadius: 100,
    fontSize: 12, fontWeight: 500, flexShrink: 0,
    fontFamily: 'var(--font-sans, system-ui)',
  },
  patternRow: {
    display: 'flex', alignItems: 'center', justifyContent: 'space-between',
    height: '100%', padding: '0 16px',
    borderBottom: '1px solid var(--border)',
  },
  patternText: {
    fontSize: 13, color: 'var(--text-primary)',
    fontFamily: 'var(--font-mono)',
    overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const,
  },
  iconBtn: {
    background: 'none', border: 'none',
    color: 'var(--text-tertiary)', cursor: 'pointer',
    padding: 4, display: 'flex', alignItems: 'center',
    borderRadius: 4,
  },
  dropMenu: {
    background: 'var(--bg-surface-opaque-2)',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-md, 8px)',
    boxShadow: '0 8px 24px rgba(0,0,0,0.5)',
    zIndex: 2000, width: 150, padding: '4px 0',
  },
  dropItem: {
    display: 'flex', alignItems: 'center', width: '100%',
    background: 'none', border: 'none',
    color: 'var(--text-primary)', fontSize: 13,
    padding: '7px 12px', cursor: 'pointer',
    fontFamily: 'var(--font-sans)', textAlign: 'left' as const,
  },
  confirmOverlay: {
    marginTop: 12,
    background: 'var(--bg-surface-2, var(--bg-surface))',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-md, 8px)', padding: 14,
  },
  confirmText: {
    fontSize: 14, color: 'var(--text-primary)',
    marginBottom: 10, fontFamily: 'var(--font-sans)',
  },
  confirmBtns: {
    display: 'flex', gap: 8, flexWrap: 'wrap' as const,
  },
  dangerBtn: {
    background: '#e53935', border: 'none',
    borderRadius: 'var(--radius-sm, 4px)',
    color: '#fff', fontSize: 12, padding: '6px 12px',
    cursor: 'pointer', fontFamily: 'var(--font-sans)',
  },
  secondaryBtn: {
    background: 'transparent',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-sm, 4px)',
    color: 'var(--text-secondary)', fontSize: 12,
    padding: '6px 12px', cursor: 'pointer',
    fontFamily: 'var(--font-sans)',
  },
};
