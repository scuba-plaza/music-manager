import { get, query } from '../api/client';
import { subscribe } from '../api/events';
import type { InfoView, JobView, LibraryResponse } from '../api/types.gen';
import { change, loadDetail } from './detail';
import { fail } from './status';
import { useApp } from './store';
import { ancestorKeys, rowByKey, trackRowKey } from './tree';

export async function loadInfo(): Promise<void> {
  try {
    useApp.setState({ info: await get<InfoView>('/api/info') });
  } catch (err) {
    fail(err);
  }
}

function followMoves(expanded: Set<string>, cursor: string | null, moves: Record<string, string>) {
  const next = new Set(expanded);
  let at = cursor;
  for (const [from, to] of Object.entries(moves)) {
    if (next.delete(from)) {
      next.add(to);
    }
    if (at === from) {
      at = to;
    }
  }
  return { expanded: next, cursor: at };
}

function revealTrack(lib: LibraryResponse, expanded: Set<string>, cursor: string | null, id: string) {
  const current = rowByKey(lib, cursor);
  const row = current?.trackId === id ? current : rowByKey(lib, trackRowKey(id));
  if (row === undefined) {
    return { expanded, cursor };
  }
  return { expanded: new Set([...expanded, ...ancestorKeys(lib.view.rows, row)]), cursor: row.key };
}

function withoutGoneTracks(marks: Set<string>, lib: LibraryResponse): Set<string> {
  const have = new Set(lib.view.rows.flatMap((row) => (row.trackId === undefined ? [] : [row.trackId])));
  const kept = [...marks].filter((id) => have.has(id));
  return kept.length === marks.size ? marks : new Set(kept);
}

let librarySeq = 0;

export interface LibraryOptions {
  moves?: Record<string, string> | undefined;
  reveal?: string | undefined;
}

export async function loadLibrary(options: LibraryOptions = {}): Promise<void> {
  librarySeq += 1;
  const seq = librarySeq;
  const { filter, sort } = useApp.getState();
  try {
    const lib = await get<LibraryResponse>(
      '/api/library' + query({ q: filter.trim(), sort: sort.key, reverse: sort.key !== '' && sort.reverse }),
    );
    if (seq !== librarySeq) {
      return;
    }
    const state = useApp.getState();
    const followed = options.moves === undefined ? state : followMoves(state.expanded, state.cursor, options.moves);
    const revealed = options.reveal === undefined ? followed : revealTrack(lib, followed.expanded, followed.cursor, options.reveal);
    change({
      lib,
      sort: { key: lib.view.sort.key, reverse: lib.view.sort.reverse },
      marks: filter.trim() === '' ? withoutGoneTracks(state.marks, lib) : state.marks,
      expanded: revealed.expanded,
      cursor: revealed.cursor,
    });
    await loadDetail();
  } catch (err) {
    if (seq === librarySeq) {
      fail(err);
    }
  }
}

export function connect(onJob: (job: JobView) => void): () => void {
  return subscribe({
    library(stamp) {
      const { lib, busy } = useApp.getState();
      if (stamp !== lib?.stamp && !busy) {
        void loadLibrary();
      }
    },
    job: onJob,
    open() {
      if (useApp.getState().lib) {
        void loadLibrary();
      }
    },
  });
}
