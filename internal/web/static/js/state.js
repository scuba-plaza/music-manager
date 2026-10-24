// The state of the page. It is plain data; main.js changes it and asks for
// the parts of the page that depend on it to be drawn again.
export const S = {
  info: null,        // what does not change: root, capabilities
  lib: null,         // the library view from the server: rows, counts, sort, playlists
  stamp: '',
  unsynced: { n: 0, known: false },
  filter: '',
  sort: { key: '', reverse: false },
  expanded: new Set(JSON.parse(safeGet('mm.expanded') || '[]')),
  cursor: null,      // the key of the row under the cursor
  marks: new Set(),  // IDs of marked tracks
  focus: 'tree',     // tree or inspector
  field: 0,          // the field under the cursor in the inspector
  tab: Number(safeGet('mm.tab') || 0),
  detail: null,      // what the right pane shows for the cursor: {kind: 'track'|'peek', ...}
  status: { text: '', kind: '' },
  overlay: null,
  busy: false,
};

export function safeGet(key) {
  try { return localStorage.getItem(key); } catch { return null; }
}
export function safeSet(key, value) {
  try { localStorage.setItem(key, value); } catch { /* private window: nothing remembered */ }
}
export function saveExpanded() { safeSet('mm.expanded', JSON.stringify([...S.expanded])); }
export function saveTab() { safeSet('mm.tab', String(S.tab)); }
