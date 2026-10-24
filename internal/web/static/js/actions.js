import { S, saveTab } from './state.js';
import { get, query } from './api.js';
import { h, clear, plural } from './dom.js';
import * as tree from './tree.js';
import * as inspector from './inspector.js';
import { openHelp } from './help.js';
import { openSortMenu } from './sortmenu.js';

// Everything the person can do, one function each. keys.js and the mouse
// handlers in main.js call these.

const $ = (id) => document.getElementById(id);

// ---- status line

export function setStatus(text, kind = '') {
  S.status = { text, kind };
  const el = $('status');
  el.textContent = text;
  el.className = kind;
}
export const fail = (err) => setStatus(err.message || String(err), 'bad');
export const clearStatus = () => { if (S.status.text) setStatus(''); };

// ---- header, titles and footer

export function renderHeader() {
  if (!S.lib) return;
  $('root').textContent = S.info ? S.info.root : '';
  let s = plural(S.lib.tracks, 'track');
  if (S.lib.needWork > 0) s += ` · ${S.lib.needWork} need work`;
  if (S.marks.size > 0) s += ` · ${S.marks.size} marked`;
  $('summary').textContent = s;
  const chip = $('sync-chip');
  chip.hidden = !(S.unsynced.n > 0);
  chip.textContent = `${S.unsynced.n} to sync · press s`;
  $('left-title').textContent = S.lib.sort.title;
  document.title = `mm · ${S.info ? S.info.root : ''}`;
}

function key(k, d) { return [k, d]; }

// hints are the keys worth showing now.
export function currentHints() {
  if (S.overlay) return S.overlay.hints ? S.overlay.hints() : [];
  if (document.activeElement === $('filter')) return [key('enter', 'keep'), key('esc', 'clear')];
  if (S.focus === 'inspector') {
    return [key('↑↓', 'field'), key('1-4', 'tabs'), key('esc', 'back'), key('?', 'help')];
  }
  const hints = [key('↑↓', 'move'), key('→', 'open'), key('space', 'mark'), key('/', 'filter'), key('S', 'sort')];
  if (S.filter) hints.push(key('esc', 'clear filter'));
  else if (S.marks.size) hints.push(key('esc', 'clear marks'));
  hints.push(key('?', 'help'));
  return hints;
}

export function renderHints() {
  const el = clear($('hints'));
  for (const [k, d] of currentHints()) el.append(h('span', { class: 'hint' }, h('kbd', {}, k), ` ${d}`));
}

export function renderFocus() {
  $('left').classList.toggle('focus', S.focus === 'tree' && !S.overlay);
  $('right').classList.toggle('focus', S.focus === 'inspector' && !S.overlay);
  $('main').dataset.show = S.focus === 'inspector' ? 'right' : 'left';
}

// ---- loading

let librarySeq = 0;
export async function loadLibrary() {
  const seq = ++librarySeq;
  try {
    const res = await get('/api/library' + query({ q: S.filter.trim(), sort: S.sort.key, reverse: S.sort.key && S.sort.reverse }));
    if (seq !== librarySeq) return;
    S.lib = res.view;
    S.stamp = res.stamp;
    S.unsynced = res.unsynced;
    S.sort = { key: res.view.sort.key, reverse: res.view.sort.reverse };
    if (!S.filter.trim() && !S.lib.sort.key) {
      const have = new Set(S.lib.rows.filter((r) => r.trackId).map((r) => r.trackId));
      for (const id of [...S.marks]) if (!have.has(id)) S.marks.delete(id);
    }
    renderHeader();
    tree.renderTree();
    await loadDetail();
  } catch (err) {
    if (seq === librarySeq) fail(err);
  }
}

let detailSeq = 0;
let detailTimer = 0;
export function scheduleDetail() {
  clearTimeout(detailTimer);
  detailTimer = setTimeout(loadDetail, 40);
}

export async function loadDetail() {
  clearTimeout(detailTimer);
  const row = tree.cursorRow();
  const seq = ++detailSeq;
  if (!row) {
    S.detail = null;
    inspector.renderInspector();
    renderHints();
    return;
  }
  try {
    let detail;
    if (row.kind === 'track') detail = { kind: 'track', id: row.trackId, data: await get(`/api/tracks/${encodeURIComponent(row.trackId)}`) };
    else detail = { kind: 'peek', data: await get('/api/peek' + query({ key: row.key, q: S.filter.trim() })) };
    if (seq !== detailSeq) return;
    S.detail = detail;
  } catch (err) {
    if (seq !== detailSeq) return;
    S.detail = { kind: 'error', message: err.message };
  }
  const max = S.detail.kind === 'track' ? inspector.fieldsOfTab(S.detail.data).length - 1 : 0;
  S.field = Math.min(S.field, Math.max(max, 0));
  inspector.renderInspector();
  renderHints();
}

tree.hooks.cursorChanged = () => { scheduleDetail(); renderHints(); };
tree.hooks.structureChanged = () => renderHints();
inspector.hooks.gotoKey = (k) => tree.reveal(k);
inspector.hooks.tabChosen = () => { inspector.renderInspector(); renderHints(); };
inspector.hooks.fieldChosen = (i) => { S.focus = 'inspector'; S.field = i; renderFocus(); inspector.renderInspector(); renderHints(); };

// ---- moving

export const move = (n) => tree.moveBy(n);
export const page = (dir) => tree.moveBy(dir * tree.pageSize());

export function open() {
  const row = tree.cursorRow();
  if (!row) return;
  if (row.kind === 'track') return focusInspector();
  if (!tree.isOpen(row)) tree.toggle(row, true);
  else {
    const child = tree.firstChild(row);
    if (child) tree.setCursor(child.key);
  }
}

export function close() {
  const row = tree.cursorRow();
  if (!row) return;
  if (row.kind !== 'track' && tree.isOpen(row) && !tree.filtering() && !tree.flatList()) {
    tree.toggle(row, false);
    return;
  }
  const parent = tree.parentOf(row);
  if (parent && !tree.flatList()) tree.setCursor(parent.key);
}

export function enter() {
  const row = tree.cursorRow();
  if (!row) return;
  if (row.kind === 'track') focusInspector();
  else tree.toggle(row);
}

export function focusInspector() {
  const row = tree.cursorRow();
  if (!row || row.kind !== 'track' || !S.detail || S.detail.kind !== 'track') return;
  S.focus = 'inspector';
  renderFocus();
  inspector.renderInspector();
  renderHints();
}

export function focusTree() {
  S.focus = 'tree';
  renderFocus();
  inspector.renderInspector();
  renderHints();
}

export function toggleFocus() {
  if (S.focus === 'tree') focusInspector();
  else focusTree();
}

// ---- the inspector

export function moveField(n) {
  if (!S.detail || S.detail.kind !== 'track') return;
  const count = inspector.fieldsOfTab(S.detail.data).length;
  S.field = Math.min(Math.max(S.field + n, 0), count - 1);
  inspector.renderInspector();
}

export function chooseTab(i) {
  if (!S.detail || S.detail.kind !== 'track') return;
  const n = S.detail.data.tabs.length;
  S.tab = ((i % n) + n) % n;
  saveTab();
  S.field = 0;
  inspector.renderInspector();
  renderHints();
}
export const nextTab = (d) => chooseTab(inspector.tabIndex(S.detail.data) + d);

// ---- marks

export function toggleMark() {
  const row = tree.cursorRow();
  if (!row) return;
  const ids = tree.idsOf(row);
  if (ids.length === 0) return;
  const all = ids.every((id) => S.marks.has(id));
  for (const id of ids) all ? S.marks.delete(id) : S.marks.add(id);
  renderHeader();
  tree.renderTree();
  tree.moveBy(1);
}

export function markAll() {
  const ids = [...new Set(tree.visible().filter((r) => r.trackId).map((r) => r.trackId))];
  const all = ids.length > 0 && ids.every((id) => S.marks.has(id));
  for (const id of ids) all ? S.marks.delete(id) : S.marks.add(id);
  renderHeader();
  tree.renderTree();
}

export function escape() {
  if (S.focus === 'inspector') return focusTree();
  if (S.filter) return setFilter('');
  if (S.marks.size) {
    S.marks.clear();
    renderHeader();
    tree.renderTree();
  }
}

// ---- filter and sort

export function focusFilter() {
  $('filter').focus();
  $('filter').select();
  renderHints();
}

let filterTimer = 0;
export function typedFilter() {
  S.filter = $('filter').value;
  clearTimeout(filterTimer);
  filterTimer = setTimeout(() => { S.cursorBeforeFilter = S.cursor; loadLibrary(); }, 150);
}

export function setFilter(text) {
  $('filter').value = text;
  S.filter = text;
  clearTimeout(filterTimer);
  loadLibrary();
}

export function sortMenu() {
  if (!S.lib) return;
  openSortMenu(async (keyName, reverse) => {
    // a list has only tracks: take the first one under the row
    const row = tree.cursorRow();
    if (keyName && row && row.kind !== 'track') {
      const ids = tree.idsOf(row);
      if (ids.length) S.cursor = `track\x1f${ids[0]}`;
    }
    const wasTrack = row && row.kind === 'track' ? row : null;
    S.sort = { key: keyName, reverse };
    await loadLibrary();
    if (!keyName && wasTrack) tree.reveal(wasTrack.key, null);
  });
}

export const help = openHelp;

export function notYet(what) {
  setStatus(`${what} is not built into the web interface yet.`, 'bad');
}
