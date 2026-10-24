import { S } from './state.js';
import * as a from './actions.js';
import * as tree from './tree.js';

// The keys, as the terminal interface had them. A key means different things
// in the tree and in the inspector; the footer hints follow the same rules.

function keyName(e) {
  if (e.ctrlKey) return 'ctrl+' + e.key.toLowerCase();
  return e.key;
}

const common = {
  '?': a.help,
  S: a.sortMenu,
  '/': a.focusFilter,
  Tab: a.toggleFocus,
  Escape: a.escape,
};

const notYet = (what) => () => a.notYet(what);

const treeKeys = {
  ArrowUp: () => a.move(-1), k: () => a.move(-1),
  ArrowDown: () => a.move(1), j: () => a.move(1),
  PageUp: () => a.page(-1), 'ctrl+u': () => a.page(-1),
  PageDown: () => a.page(1), 'ctrl+d': () => a.page(1),
  Home: () => tree.moveTo('top'), g: () => tree.moveTo('top'),
  End: () => tree.moveTo('bottom'), G: () => tree.moveTo('bottom'),
  z: tree.centre,
  ArrowRight: a.open, l: a.open,
  ArrowLeft: a.close, h: a.close,
  Enter: a.enter,
  '+': tree.openLevel, '=': tree.openLevel,
  '-': tree.closeAll,
  ' ': a.toggleMark,
  'ctrl+a': a.markAll,
  e: notYet('Editing'), c: notYet('Copy'), v: notYet('Paste'), d: notYet('Removing'),
  J: notYet('Moving a track'), K: notYet('Moving a track'), n: notYet('Numbering'), p: notYet('Playlists'),
  i: notYet('Identify'), I: notYet('Identify'), o: notYet('Adopting old tags'),
  a: notYet('Adding'), s: notYet('Sync'), D: notYet('The library check'),
  u: notYet('Undo'), 'ctrl+r': notYet('Redo'),
};

const inspectorKeys = {
  ArrowUp: () => a.moveField(-1), k: () => a.moveField(-1),
  ArrowDown: () => a.moveField(1), j: () => a.moveField(1),
  ArrowLeft: () => (S.tab === 0 ? a.focusTree() : a.nextTab(-1)), h: () => (S.tab === 0 ? a.focusTree() : a.nextTab(-1)),
  ArrowRight: () => a.nextTab(1), l: () => a.nextTab(1),
  1: () => a.chooseTab(0), 2: () => a.chooseTab(1), 3: () => a.chooseTab(2), 4: () => a.chooseTab(3),
  Enter: notYet('Editing'), c: notYet('Copy'), v: notYet('Paste'), p: notYet('Playing'),
  '[': notYet('Nudging'), ']': notYet('Nudging'), '{': notYet('Nudging'), '}': notYet('Nudging'), 0: notYet('Clearing'),
  x: notYet('Removing the cover'), i: notYet('Identify'), o: notYet('Adopting old tags'),
  a: notYet('Adding'), s: notYet('Sync'), D: notYet('The library check'), u: notYet('Undo'), 'ctrl+r': notYet('Redo'),
};

export function onKeydown(e) {
  if (e.metaKey || e.altKey || e.isComposing) return;
  if (S.overlay) {
    if (S.overlay.onKey(e)) e.preventDefault();
    return;
  }
  const el = document.activeElement;
  if (el && el.id === 'filter') return filterKey(e);
  if (el && /^(INPUT|TEXTAREA|SELECT)$/.test(el.tagName)) return;
  a.clearStatus();
  const k = keyName(e);
  const handler = (S.focus === 'inspector' ? inspectorKeys[k] : treeKeys[k]) || common[k];
  if (handler) {
    e.preventDefault();
    handler();
  }
}

function filterKey(e) {
  const box = document.getElementById('filter');
  if (e.key === 'Enter' || e.key === 'ArrowDown') {
    e.preventDefault();
    box.blur();
    document.getElementById('tree').focus({ preventScroll: true });
    a.renderHints();
  } else if (e.key === 'Escape') {
    e.preventDefault();
    box.blur();
    document.getElementById('tree').focus({ preventScroll: true });
    a.setFilter('');
    a.renderHints();
  }
}
