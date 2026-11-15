import { get } from '../api/client';
import type { CopyChoice } from '../api/types.gen';
import { fail, setStatus } from '../state/status';
import { useApp } from '../state/store';
import { idsOf, markedIds, rowByKey } from '../state/tree';

export function openPlaylistPicker(): void {
  const { lib, cursor, marks } = useApp.getState();
  const row = rowByKey(lib, cursor);
  let ids: string[] = [];
  if (lib !== null && marks.size > 0) {
    ids = markedIds(lib.view.rows, marks);
  } else if (lib !== null && row !== undefined && row.key !== 'playlists') {
    ids = idsOf(lib.view.rows, row);
  }
  if (ids.length === 0) {
    setStatus('Select tracks to add to a playlist: a track, an album, or mark several with space', 'bad');
    return;
  }
  useApp.setState({ overlay: { kind: 'playlist', ids } });
}

export async function openCopyList(): Promise<void> {
  const { lib, cursor } = useApp.getState();
  const row = rowByKey(lib, cursor);
  if (row?.kind !== 'track' || row.trackId === undefined) {
    setStatus('Select a track to copy from', 'bad');
    return;
  }
  try {
    const choices = await get<CopyChoice[]>(`/api/tracks/${encodeURIComponent(row.trackId)}/copyable`);
    if (useApp.getState().overlay === null) {
      useApp.setState({ overlay: { kind: 'copy', id: row.trackId, title: row.label, choices } });
    }
  } catch (err) {
    fail(err);
  }
}

export function openHelp(): void {
  useApp.setState({ overlay: { kind: 'help' } });
}

export function openSortMenu(): void {
  if (useApp.getState().lib !== null) {
    useApp.setState({ overlay: { kind: 'sort' } });
  }
}
