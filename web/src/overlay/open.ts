import { get, send } from '../api/client';
import type { ChangeResponse, CopyChoice } from '../api/types.gen';
import type { ActionName } from '../keys/keymap';
import { fail, loadLibrary, setStatus } from '../state/actions';
import { applyChange } from '../state/edit';
import { useApp, type AppState } from '../state/store';
import { idsOf, markedIds, rowByKey, trackRowKey } from '../state/tree';

export function closeOverlay(): void {
  useApp.setState({ overlay: null });
}

function message(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

async function change(path: string, body: unknown): Promise<string> {
  try {
    await applyChange(() => send<ChangeResponse>('POST', path, body));
  } catch (err) {
    return message(err);
  }
  closeOverlay();
  return '';
}

export function addToPlaylist(name: string, isNew: boolean, ids: readonly string[]): Promise<string> {
  return change(isNew ? '/api/playlists' : '/api/playlists/add', { name, ids });
}

export function copyAttributes(id: string, attrs: readonly string[]): Promise<string> {
  return change('/api/copy', { id, attrs });
}

export async function chooseSort(key: string, reverse: boolean): Promise<void> {
  const { lib, cursor } = useApp.getState();
  const row = rowByKey(lib, cursor);
  const patch: Partial<AppState> = { sort: { key, reverse } };
  const first = lib === null || row === undefined ? undefined : idsOf(lib.view.rows, row)[0];
  if (key !== '' && first !== undefined) {
    patch.cursor = trackRowKey(first);
  }
  useApp.setState(patch);
  await loadLibrary({ reveal: key === '' && row?.kind === 'track' ? row.trackId : undefined });
}

function openPlaylistPicker(): void {
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

async function openCopyList(): Promise<void> {
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

export function opensOverlay(action: ActionName): boolean {
  const { lib, focus } = useApp.getState();
  switch (action) {
    case 'help':
      useApp.setState({ overlay: { kind: 'help' } });
      return true;
    case 'sortMenu':
      if (lib !== null) {
        useApp.setState({ overlay: { kind: 'sort' } });
      }
      return true;
    case 'addToPlaylist':
      openPlaylistPicker();
      return true;
    case 'copy':
      if (focus !== 'tree') {
        return false;
      }
      void openCopyList();
      return true;
    default:
      return false;
  }
}
