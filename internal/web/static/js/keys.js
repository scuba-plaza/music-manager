import { S } from './state.js';
import * as a from './actions.js';
import * as tree from './tree.js';
import * as e from './edits.js';
import * as ops from './ops.js';
import * as sync from './sync.js';

// The keys, as the terminal interface had them. A key means different things
// in the tree and in the inspector; the footer hints follow the same rules.

function keyName(ev) {
  if (ev.ctrlKey) return 'ctrl+' + ev.key.toLowerCase();
  return ev.key;
}

const common = {
  '?': a.help,
  S: a.sortMenu,
  '/': a.focusFilter,
  Tab: a.toggleFocus,
  Escape: a.escape,
};

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
  e: e.bulkEdit, c: e.copyKey, v: e.paste, d: e.remove, Delete: e.remove,
  J: () => e.moveTrack(1), K: () => e.moveTrack(-1), n: e.renumber, p: e.playlistKey,
  i: ops.identify, I: ops.identifyAll, o: ops.adopt,
  a: ops.add, s: sync.openSync, D: ops.doctor,
  u: e.undo, 'ctrl+r': e.redo,
};

// a track has tabs to turn to; an artist, an album or a playlist has only its fields
const onTrack = () => S.detail && S.detail.kind === 'track';
const left = () => (onTrack() && S.tab > 0 ? a.nextTab(-1) : a.focusTree());
const right = () => { if (onTrack()) a.nextTab(1); };
const tab = (i) => () => { if (onTrack()) a.chooseTab(i); };

const inspectorKeys = {
  ArrowUp: () => a.moveField(-1), k: () => a.moveField(-1),
  ArrowDown: () => a.moveField(1), j: () => a.moveField(1),
  ArrowLeft: left, h: left, ArrowRight: right, l: right,
  1: tab(0), 2: tab(1), 3: tab(2), 4: tab(3),
  Enter: e.startEdit, c: e.copyKey, v: e.paste, p: e.play,
  '[': () => e.nudge(-1, false), ']': () => e.nudge(1, false), '{': () => e.nudge(-1, true), '}': () => e.nudge(1, true), 0: e.clearField,
  x: e.removeCover, i: ops.identify, o: ops.adopt,
  a: ops.add, s: sync.openSync, D: ops.doctor, u: e.undo, 'ctrl+r': e.redo,
};

export function onKeydown(ev) {
  if (ev.metaKey || ev.altKey || ev.isComposing) return;
  if (S.overlay) {
    if (S.overlay.onKey(ev)) ev.preventDefault();
    return;
  }
  const el = document.activeElement;
  if (el && el.id === 'filter') return filterKey(ev);
  if (el && /^(INPUT|TEXTAREA|SELECT)$/.test(el.tagName)) return;
  const k = keyName(ev);
  const handler = (S.focus === 'inspector' ? inspectorKeys[k] : treeKeys[k]) || common[k];
  if (!handler) return;
  ev.preventDefault();
  if (S.busy) return; // a change is on its way: keys wait, as they did in the terminal
  a.clearStatus();
  handler();
}

function filterKey(ev) {
  const box = document.getElementById('filter');
  if (ev.key === 'Enter' || ev.key === 'ArrowDown') {
    ev.preventDefault();
    box.blur();
    document.getElementById('tree').focus({ preventScroll: true });
    a.renderHints();
  } else if (ev.key === 'Escape') {
    ev.preventDefault();
    box.blur();
    document.getElementById('tree').focus({ preventScroll: true });
    a.setFilter('');
    a.renderHints();
  }
}
