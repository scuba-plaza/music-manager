import type { RowView } from '../api/types.gen';
import type { ActionName } from '../keys/keymap';
import { opensOverlay } from '../overlay/open';
import { change } from './detail';
import { runInspector, stopPlaying } from './edit';
import { loadLibrary } from './library';
import { runLookup } from './lookup';
import { runOps } from './ops';
import { setStatus } from './status';
import { useApp, type Focus } from './store';
import {
  canFold,
  idsOf,
  isFlat,
  isFolder,
  isOpen,
  keysToOpenOneLevel,
  rowByKey,
  visibleRows,
} from './tree';
import { centreCursor, focusFilterBox, leaveFilterBox, pageSize } from './treeDom';

const filterPause = 150;

function focusPane(focus: Focus): void {
  useApp.setState({ focus });
}

function cursorRow(): RowView | undefined {
  const { lib, cursor } = useApp.getState();
  return rowByKey(lib, cursor);
}

function setCursor(key: string): void {
  change({ cursor: key });
}

function moveBy(n: number): void {
  const shown = visibleRows(useApp.getState());
  if (shown.length === 0) {
    return;
  }
  const cursor = useApp.getState().cursor;
  const at = Math.max(
    0,
    shown.findIndex((row) => row.key === cursor),
  );
  const next = shown[Math.min(Math.max(at + n, 0), shown.length - 1)];
  if (next) {
    setCursor(next.key);
  }
}

function moveToEnd(end: 'top' | 'bottom'): void {
  const shown = visibleRows(useApp.getState());
  const row = end === 'top' ? shown[0] : shown[shown.length - 1];
  if (row) {
    setCursor(row.key);
  }
}

function fold(row: RowView, open: boolean): void {
  const state = useApp.getState();
  if (!isFolder(row) || !canFold(state)) {
    return;
  }
  const expanded = new Set(state.expanded);
  if (open) {
    expanded.add(row.key);
  } else {
    expanded.delete(row.key);
  }
  change({ expanded });
}

export function enterFields(row: RowView): boolean {
  if (row.kind !== 'track' && row.editable !== true) {
    return false;
  }
  useApp.setState(row.kind === 'track' ? { focus: 'inspector' } : { focus: 'inspector', field: 0 });
  return true;
}

function openRow(): void {
  const row = cursorRow();
  if (row === undefined || enterFields(row)) {
    return;
  }
  const state = useApp.getState();
  if (!isOpen(state, row)) {
    fold(row, true);
    return;
  }
  const shown = visibleRows(state);
  const next = shown[shown.indexOf(row) + 1];
  if (next && next.depth > row.depth) {
    setCursor(next.key);
  }
}

function closeRow(): void {
  const row = cursorRow();
  if (row === undefined) {
    return;
  }
  const state = useApp.getState();
  if (isFolder(row) && isOpen(state, row) && canFold(state)) {
    fold(row, false);
    return;
  }
  const parent = rowByKey(state.lib, row.parent ?? null);
  if (parent && !isFlat(state.lib)) {
    setCursor(parent.key);
  }
}

function enterRow(): void {
  const row = cursorRow();
  if (row === undefined) {
    return;
  }
  if (row.kind === 'track') {
    enterFields(row);
  } else {
    fold(row, !isOpen(useApp.getState(), row));
  }
}

function openOneLevel(): void {
  const state = useApp.getState();
  const keys = keysToOpenOneLevel(state);
  if (keys.length > 0) {
    change({ expanded: new Set([...state.expanded, ...keys]) });
  }
}

function closeEverything(): void {
  const state = useApp.getState();
  if (canFold(state) && state.expanded.size > 0) {
    change({ expanded: new Set() });
  }
}

function toggledMarks(marks: ReadonlySet<string>, ids: readonly string[]): Set<string> {
  const all = ids.length > 0 && ids.every((id) => marks.has(id));
  const next = new Set(marks);
  for (const id of ids) {
    if (all) {
      next.delete(id);
    } else {
      next.add(id);
    }
  }
  return next;
}

function toggleMark(): void {
  const row = cursorRow();
  const { lib, marks } = useApp.getState();
  if (row === undefined || lib === null) {
    return;
  }
  const ids = idsOf(lib.view.rows, row);
  if (ids.length === 0) {
    return;
  }
  useApp.setState({ marks: toggledMarks(marks, ids) });
  moveBy(1);
}

function markAll(): void {
  const state = useApp.getState();
  const ids = new Set(visibleRows(state).flatMap((row) => (row.trackId === undefined ? [] : [row.trackId])));
  if (ids.size > 0) {
    useApp.setState({ marks: toggledMarks(state.marks, [...ids]) });
  }
}

let filterTimer: ReturnType<typeof setTimeout> | undefined;
let composing = false;

function scheduleFilterLoad(): void {
  clearTimeout(filterTimer);
  filterTimer = setTimeout(() => {
    void loadLibrary();
  }, filterPause);
}

export function typeFilter(text: string): void {
  change({ filter: text });
  if (!composing) {
    scheduleFilterLoad();
  }
}

export function composeFilter(active: boolean): void {
  composing = active;
  if (!active) {
    scheduleFilterLoad();
  }
}

export function setFilter(text: string): void {
  clearTimeout(filterTimer);
  change({ filter: text });
  void loadLibrary();
}

export function setFilterFocused(filterFocused: boolean): void {
  useApp.setState({ filterFocused });
}

export function pointRow(key: string): void {
  useApp.setState({ focus: 'tree', edit: null });
  setCursor(key);
}

export function foldRow(key: string): void {
  pointRow(key);
  const row = cursorRow();
  if (row !== undefined) {
    fold(row, !isOpen(useApp.getState(), row));
  }
}

export function markRow(key: string): void {
  pointRow(key);
  toggleMark();
}

function toggleFocus(focus: Focus): void {
  const row = cursorRow();
  if (focus === 'inspector') {
    focusPane('tree');
  } else if (row !== undefined) {
    enterFields(row);
  }
}

function escape(): void {
  const { focus, filter, marks } = useApp.getState();
  if (stopPlaying()) {
    return;
  }
  if (focus === 'inspector') {
    focusPane('tree');
  } else if (filter !== '') {
    setFilter('');
  } else if (marks.size > 0) {
    useApp.setState({ marks: new Set() });
  }
}

export function run(action: ActionName): void {
  const { focus, status } = useApp.getState();
  if (status.text !== '') {
    setStatus('');
  }
  switch (action) {
    case 'moveUp':
      moveBy(-1);
      break;
    case 'moveDown':
      moveBy(1);
      break;
    case 'pageUp':
      moveBy(-pageSize());
      break;
    case 'pageDown':
      moveBy(pageSize());
      break;
    case 'moveToTop':
      moveToEnd('top');
      break;
    case 'moveToBottom':
      moveToEnd('bottom');
      break;
    case 'centre':
      centreCursor();
      break;
    case 'open':
      openRow();
      break;
    case 'close':
      closeRow();
      break;
    case 'enterRow':
      enterRow();
      break;
    case 'openLevel':
      openOneLevel();
      break;
    case 'closeAll':
      closeEverything();
      break;
    case 'markToggle':
      toggleMark();
      break;
    case 'markAll':
      markAll();
      break;
    case 'focusFilter':
      focusFilterBox();
      break;
    case 'filterAccept':
      leaveFilterBox();
      break;
    case 'filterClear':
      leaveFilterBox();
      setFilter('');
      break;
    case 'escape':
      escape();
      break;
    case 'toggleFocus':
      toggleFocus(focus);
      break;
    default:
      if (!opensOverlay(action) && !runOps(action) && !runLookup(action)) {
        runInspector(action);
      }
      break;
  }
}

export function takeTreeFocus(): void {
  if (useApp.getState().focus !== 'tree') {
    focusPane('tree');
  }
}
