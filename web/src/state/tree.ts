import type { LibraryResponse, RowView } from '../api/types.gen';
import type { AppState } from './store';

type Rows = Pick<AppState, 'lib' | 'filter' | 'expanded'>;

export type MarkState = 'none' | 'some' | 'all';

export function isFiltering(filter: string): boolean {
  return filter.trim() !== '';
}

export function isFlat(lib: LibraryResponse | null): boolean {
  return lib !== null && lib.view.sort.key !== '';
}

export function isFolder(row: RowView): boolean {
  return row.kind !== 'track';
}

export function canFold(state: Rows): boolean {
  return !isFiltering(state.filter) && !isFlat(state.lib);
}

export function isOpen(state: Pick<AppState, 'filter' | 'expanded'>, row: RowView): boolean {
  return isFiltering(state.filter) || state.expanded.has(row.key);
}

export function visibleRows(state: Rows): RowView[] {
  if (state.lib === null) {
    return [];
  }
  const rows = state.lib.view.rows;
  if (isFlat(state.lib)) {
    return rows;
  }
  const shown: RowView[] = [];
  let hiddenBelow = Infinity;
  for (const row of rows) {
    if (row.depth > hiddenBelow) {
      continue;
    }
    hiddenBelow = Infinity;
    shown.push(row);
    if (isFolder(row) && !isOpen(state, row)) {
      hiddenBelow = row.depth;
    }
  }
  return shown;
}

export function rowByKey(lib: LibraryResponse | null, key: string | null): RowView | undefined {
  return key === null ? undefined : lib?.view.rows.find((row) => row.key === key);
}

function rowsBelow(rows: readonly RowView[], row: RowView): RowView[] {
  const start = rows.indexOf(row);
  if (start < 0) {
    return [];
  }
  const below: RowView[] = [];
  for (let i = start + 1; i < rows.length; i += 1) {
    const next = rows[i];
    if (next === undefined || next.depth <= row.depth) {
      break;
    }
    below.push(next);
  }
  return below;
}

export function idsOf(rows: readonly RowView[], row: RowView): string[] {
  if (row.kind === 'track') {
    return row.trackId === undefined ? [] : [row.trackId];
  }
  const ids = new Set<string>();
  for (const below of rowsBelow(rows, row)) {
    if (below.trackId !== undefined) {
      ids.add(below.trackId);
    }
  }
  return [...ids];
}

export function markStateOf(rows: readonly RowView[], row: RowView, marks: ReadonlySet<string>): MarkState {
  const ids = idsOf(rows, row);
  const marked = ids.filter((id) => marks.has(id)).length;
  if (marked === 0) {
    return 'none';
  }
  return marked === ids.length ? 'all' : 'some';
}

export function keysToOpenOneLevel(state: Rows): string[] {
  if (!canFold(state)) {
    return [];
  }
  const closed = visibleRows(state).filter((row) => isFolder(row) && !isOpen(state, row));
  const depth = Math.min(...closed.map((row) => row.depth));
  return closed.filter((row) => row.depth === depth).map((row) => row.key);
}

export function ancestorKeys(rows: readonly RowView[], row: RowView): string[] {
  const byKey = new Map(rows.map((candidate) => [candidate.key, candidate]));
  const keys: string[] = [];
  for (let parent = byKey.get(row.parent ?? ''); parent !== undefined; parent = byKey.get(parent.parent ?? '')) {
    keys.push(parent.key);
  }
  return keys;
}

export function settledCursor(state: Rows & Pick<AppState, 'cursor'>): string | null {
  const shown = visibleRows(state);
  const shownKeys = new Set(shown.map((row) => row.key));
  const current = rowByKey(state.lib, state.cursor);
  if (current === undefined) {
    return shown[0]?.key ?? null;
  }
  if (shownKeys.has(current.key)) {
    return current.key;
  }
  const rows = state.lib?.view.rows ?? [];
  return ancestorKeys(rows, current).find((key) => shownKeys.has(key)) ?? shown[0]?.key ?? null;
}

export function trackRowKey(id: string): string {
  return `track\u001f${id}`;
}

export function markedIds(rows: readonly RowView[], marks: ReadonlySet<string>): string[] {
  const ids = new Set<string>();
  for (const row of rows) {
    if (row.trackId !== undefined && marks.has(row.trackId)) {
      ids.add(row.trackId);
    }
  }
  return [...ids, ...[...marks].filter((id) => !ids.has(id))];
}
