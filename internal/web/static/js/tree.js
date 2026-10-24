import { S, saveExpanded } from './state.js';
import { h, name, clear } from './dom.js';

// The server sends the tree flat, every row open, in depth-first order. What
// is folded is decided here, so that folding costs no request.

export const hooks = { cursorChanged() {}, structureChanged() {} };

const treeEl = () => document.getElementById('tree');

export const filtering = () => S.filter.trim() !== '';
export const flatList = () => !!(S.lib && S.lib.sort.key);
export const isOpen = (row) => filtering() || S.expanded.has(row.key);

// visible lists the rows that are not hidden inside a folded one.
export function visible() {
  if (!S.lib) return [];
  if (flatList()) return S.lib.rows;
  const out = [];
  let skip = Infinity;
  for (const r of S.lib.rows) {
    if (r.depth > skip) continue;
    skip = Infinity;
    out.push(r);
    if (r.kind !== 'track' && !isOpen(r)) skip = r.depth;
  }
  return out;
}

export function rowByKey(key) {
  return S.lib && S.lib.rows.find((r) => r.key === key);
}

// below lists the rows inside a row, whether folded or not.
export function below(row) {
  const rows = S.lib.rows;
  const i = rows.indexOf(row);
  const out = [];
  for (let j = i + 1; j < rows.length && rows[j].depth > row.depth; j++) out.push(rows[j]);
  return out;
}

// idsOf lists the tracks a row stands for, in order and without repeats.
export function idsOf(row) {
  if (row.kind === 'track') return [row.trackId];
  const seen = new Set();
  const ids = [];
  for (const r of below(row)) {
    if (r.trackId && !seen.has(r.trackId)) {
      seen.add(r.trackId);
      ids.push(r.trackId);
    }
  }
  return ids;
}

export function markState(row) {
  const ids = idsOf(row);
  if (ids.length === 0) return 0;
  const n = ids.filter((id) => S.marks.has(id)).length;
  return n === 0 ? 0 : n === ids.length ? 2 : 1;
}

function rowEl(r, shown) {
  const folder = r.kind !== 'track';
  const open = folder && isOpen(r);
  const marked = markState(r);
  const el = h('div', {
    class: `row ${r.kind}${S.cursor === r.key ? ' cursor' : ''}${marked ? ' marked' : ''}`,
    role: 'treeitem', 'data-key': r.key, 'aria-level': r.depth + 1, 'aria-selected': S.cursor === r.key ? 'true' : 'false',
    'aria-expanded': folder ? String(open) : null,
    style: flatList() ? null : `padding-left:${r.depth * 2}ch`,
  });
  el.append(h('span', { class: 'mark', 'aria-hidden': 'true' }, marked === 2 ? '●' : marked === 1 ? '◐' : ''));
  el.append(h('span', { class: 'twist', 'aria-hidden': 'true' }, folder && !filtering() ? (open ? '▾' : '▸') : ''));
  if (r.num) el.append(h('span', { class: 'num' }, r.num));
  el.append(h('span', { class: 'label' }, name(r.label)));
  if (r.detail) el.append(h('span', { class: 'detail' }, name(r.detail)));
  else el.append(h('span', { class: 'spacer' }));
  const right = folder ? String(r.count) : r.right || r.length || '';
  el.append(h('span', { class: 'right' }, right));
  return el;
}

// renderTree draws the rows again, and keeps the cursor on a row.
export function renderTree() {
  const el = clear(treeEl());
  const rows = visible();
  if (!S.lib) return;
  if (rows.length === 0) {
    el.append(h('div', { class: 'empty' }, filtering() ? 'Nothing matches the filter.' : 'The library is empty. Add tracks with mm ingest, or press a to add from a link.'));
    S.cursor = null;
    return;
  }
  if (!rows.some((r) => r.key === S.cursor)) S.cursor = rows[0].key;
  const frag = document.createDocumentFragment();
  for (const r of rows) frag.append(rowEl(r));
  el.append(frag);
  scrollToCursor();
}

export function scrollToCursor() {
  const el = cursorEl();
  if (el) el.scrollIntoView({ block: 'nearest' });
}

function cursorEl() {
  return [...treeEl().children].find((c) => c.dataset.key === S.cursor);
}

// updateCursor moves the highlight without drawing every row again.
export function updateCursor() {
  for (const c of treeEl().children) {
    const on = c.dataset.key === S.cursor;
    c.classList.toggle('cursor', on);
    c.setAttribute('aria-selected', on ? 'true' : 'false');
  }
  scrollToCursor();
}

export function setCursor(key, { notify = true } = {}) {
  if (key === S.cursor) return;
  S.cursor = key;
  updateCursor();
  if (notify) hooks.cursorChanged();
}

export function cursorRow() {
  return rowByKey(S.cursor);
}

export function moveBy(n) {
  const rows = visible();
  if (rows.length === 0) return;
  const i = Math.max(0, rows.findIndex((r) => r.key === S.cursor));
  setCursor(rows[Math.min(Math.max(i + n, 0), rows.length - 1)].key);
}

export function moveTo(end) {
  const rows = visible();
  if (rows.length) setCursor(end === 'top' ? rows[0].key : rows[rows.length - 1].key);
}

export function pageSize() {
  const t = treeEl();
  const row = t.firstElementChild;
  return Math.max(1, Math.floor(t.clientHeight / (row ? row.offsetHeight : 20)) - 1);
}

export function centre() {
  const el = cursorEl();
  if (el) el.scrollIntoView({ block: 'center' });
}

// toggle opens or closes a folder row.
export function toggle(row, open = !isOpen(row)) {
  if (row.kind === 'track' || filtering() || flatList()) return false;
  if (open) S.expanded.add(row.key);
  else S.expanded.delete(row.key);
  saveExpanded();
  renderTree();
  hooks.structureChanged();
  return true;
}

// openLevel opens every closed folder that is at the shallowest closed depth.
export function openLevel() {
  if (filtering() || flatList()) return;
  let depth = Infinity;
  for (const r of visible()) if (r.kind !== 'track' && !isOpen(r)) depth = Math.min(depth, r.depth);
  if (depth === Infinity) return;
  for (const r of visible()) if (r.kind !== 'track' && !isOpen(r) && r.depth === depth) S.expanded.add(r.key);
  saveExpanded();
  renderTree();
}

export function closeAll() {
  if (filtering() || flatList()) return;
  S.expanded.clear();
  saveExpanded();
  const top = S.lib.rows.find((r) => r.key === S.cursor);
  // keep the cursor on a row that is still there: the top row above it
  if (top && !visible().includes(top)) {
    let t = top;
    while (t && t.parent && !visible().includes(t)) t = rowByKey(t.parent);
    S.cursor = t ? t.key : null;
  }
  renderTree();
  hooks.cursorChanged();
}

// reveal opens the folders above a row so that it can be seen, and puts the cursor on it.
export function reveal(key, ancestors) {
  for (const k of ancestors || []) S.expanded.add(k);
  let r = rowByKey(key);
  // the ancestors of a track in the tree: open every parent up the chain
  for (let p = r && r.parent; p; p = (rowByKey(p) || {}).parent) S.expanded.add(p);
  saveExpanded();
  S.cursor = key;
  renderTree();
  hooks.cursorChanged();
}

export function parentOf(row) {
  return row.parent ? rowByKey(row.parent) : null;
}

export function firstChild(row) {
  const rows = visible();
  const i = rows.indexOf(row);
  const next = rows[i + 1];
  return next && next.depth > row.depth ? next : null;
}
