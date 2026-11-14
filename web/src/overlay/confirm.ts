import { send } from '../api/client';
import type { ChangeResponse } from '../api/types.gen';
import { applyChange } from '../state/edit';
import { loadLibrary } from '../state/library';
import { closeOverlay, useApp, type AppState } from '../state/store';
import { idsOf, rowByKey, trackRowKey } from '../state/tree';

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
