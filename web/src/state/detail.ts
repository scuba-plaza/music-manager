import { get, query } from '../api/client';
import type { PeekView, TrackView } from '../api/types.gen';
import { fail } from './status';
import { useApp, type AppState } from './store';
import { rowByKey, settledCursor } from './tree';

const detailPause = 40;

let detailSeq = 0;
let detailTimer: ReturnType<typeof setTimeout> | undefined;
let detailAwaited = false;

export function scheduleDetail(): void {
  clearTimeout(detailTimer);
  detailAwaited = true;
  detailTimer = setTimeout(() => {
    void loadDetail();
  }, detailPause);
}

export function dropDetail(): boolean {
  clearTimeout(detailTimer);
  detailSeq += 1;
  const awaited = detailAwaited;
  detailAwaited = false;
  return awaited;
}

export async function loadDetail(): Promise<void> {
  clearTimeout(detailTimer);
  detailSeq += 1;
  const seq = detailSeq;
  detailAwaited = true;
  const { lib, cursor, filter, profile } = useApp.getState();
  const row = rowByKey(lib, cursor);
  if (row === undefined) {
    detailAwaited = false;
    useApp.setState({ detail: null, detailError: '' });
    return;
  }
  try {
    const detail =
      row.kind === 'track'
        ? await get<TrackView>(`/api/tracks/${encodeURIComponent(row.trackId ?? '')}` + query({ profile }))
        : await get<PeekView>('/api/peek' + query({ key: row.key, q: filter.trim() }));
    if (seq === detailSeq) {
      detailAwaited = false;
      useApp.setState({ detail, detailError: '' });
    }
  } catch (err) {
    if (seq === detailSeq) {
      detailAwaited = false;
      useApp.setState({ detail: null, detailError: err instanceof Error ? err.message : String(err) });
      fail(err);
    }
  }
}

export function change(patch: Partial<AppState>): void {
  const before = useApp.getState();
  const cursor = settledCursor({ ...before, ...patch });
  const moved = cursor !== before.cursor;
  useApp.setState(moved ? { ...patch, cursor, edit: null, playing: null } : { ...patch, cursor });
  if (moved) {
    scheduleDetail();
  }
}
