import { send } from '../api/client';
import type { ChangeResponse, LibraryResponse, RowView } from '../api/types.gen';
import type { ActionName } from '../keys/keymap';
import { enterFields, setStatus } from './actions';
import { openBulk } from './bulk';
import { pastePicture } from './cover';
import { attempt } from './edit';
import { currentField, isTrack } from './inspector';
import { useApp } from './store';
import { idsOf, isAlbum, isFiltering, isFlat, markedIds, rowByKey } from './tree';

const pictureType = /^image\/(jpeg|png)$/;

function plural(n: number, word: string): string {
  return n === 1 ? `1 ${word}` : `${String(n)} ${word}s`;
}

function refuse(text: string): void {
  setStatus(text, 'bad');
}

function change(path: string, body?: unknown): void {
  void attempt(() => send<ChangeResponse>('POST', path, body));
}

function ask(head: string, body: string[], yes: string, onYes: () => void): void {
  useApp.setState({ overlay: { kind: 'ask', head, body, yes, onYes } });
}

function cursorRow(): { lib: LibraryResponse; row: RowView } | undefined {
  const { lib, cursor } = useApp.getState();
  const row = rowByKey(lib, cursor);
  return lib === null || row === undefined ? undefined : { lib, row };
}

function playlistOf(lib: LibraryResponse, row: RowView): string {
  if (row.kind === 'playlist') {
    return row.label;
  }
  const parent = row.kind === 'track' ? rowByKey(lib, row.parent ?? null) : undefined;
  return parent?.kind === 'playlist' ? parent.label : '';
}

function titles(rows: readonly RowView[]): Map<string, string> {
  const names = new Map<string, string>();
  for (const row of rows) {
    if (row.trackId !== undefined && !names.has(row.trackId)) {
      names.set(row.trackId, row.label);
    }
  }
  return names;
}

function bulkEdit(): void {
  const at = cursorRow();
  if (at === undefined) {
    return;
  }
  const { lib, row } = at;
  const { marks } = useApp.getState();
  if (row.kind === 'playlist') {
    enterFields(row);
    return;
  }
  if (row.key === 'playlists') {
    refuse('Select a playlist to edit it');
    return;
  }
  if (marks.size > 0) {
    const ids = markedIds(lib.view.rows, marks);
    openBulk(ids, `Edit ${plural(ids.length, 'marked track')}`);
    return;
  }
  if (enterFields(row)) {
    return;
  }
  const ids = idsOf(lib.view.rows, row);
  if (ids.length > 0) {
    openBulk(ids, `Edit ${plural(ids.length, 'track')}`);
  }
}

function removeHead(row: RowView, n: number, marked: boolean): string {
  if (marked) {
    return `Remove ${plural(n, 'marked track')}?`;
  }
  switch (row.kind) {
    case 'track':
      return `Remove “${row.label}”?`;
    case 'artist':
      return `Remove “${row.label}” and its ${plural(n, 'track')}?`;
    case 'album':
      return `Remove the ${plural(n, 'track')} of “${row.label}”?`;
    default:
      return `Remove everything in “${row.label}” (${plural(n, 'track')})?`;
  }
}

function removeInPlaylist(lib: LibraryResponse, row: RowView, name: string): void {
  if (row.kind === 'playlist') {
    ask(`Delete the playlist “${name}”?`, [`Its ${plural(row.count ?? 0, 'track')} stay in the library.`, 'u brings the playlist back.'], 'delete', () => {
      change('/api/playlists/delete', { name });
    });
    return;
  }
  const { marks } = useApp.getState();
  const parent = rowByKey(lib, row.parent ?? null);
  const marked = parent === undefined ? [] : idsOf(lib.view.rows, parent).filter((id) => marks.has(id));
  const ids = marked.length > 0 ? marked : row.trackId === undefined ? [] : [row.trackId];
  if (ids.length > 0) {
    change('/api/playlists/takeout', { name, ids });
  }
}

function remove(): void {
  const at = cursorRow();
  if (at === undefined) {
    return;
  }
  const { lib, row } = at;
  const list = playlistOf(lib, row);
  if (list !== '') {
    removeInPlaylist(lib, row, list);
    return;
  }
  if (row.key === 'playlists') {
    refuse('Select a playlist, or one of its tracks, to take out of it');
    return;
  }
  const { marks } = useApp.getState();
  const marked = marks.size > 0;
  const ids = marked ? markedIds(lib.view.rows, marks) : idsOf(lib.view.rows, row);
  if (ids.length === 0) {
    return;
  }
  const names = titles(lib.view.rows);
  const body = ids.length > 1 ? ids.slice(0, 5).map((id) => `• ${names.get(id) ?? id}`) : [];
  if (ids.length > 5) {
    body.push(`…and ${String(ids.length - 5)} more`);
  }
  body.push(`Nothing is deleted from disk until you sync with prune, and u brings ${ids.length > 1 ? 'them' : 'it'} back until then.`);
  ask(removeHead(row, ids.length, marked), body, 'remove', () => {
    change('/api/remove', { ids });
  });
}

function moveTrack(dir: 1 | -1): void {
  const at = cursorRow();
  if (at === undefined) {
    return;
  }
  const { lib, row } = at;
  const { filter } = useApp.getState();
  if (isFlat(lib)) {
    refuse('Moving a track needs the tree view · S, then Tree');
    return;
  }
  const list = row.kind === 'track' ? playlistOf(lib, row) : '';
  if (list !== '') {
    if (isFiltering(filter)) {
      refuse('Clear the filter first, so the whole playlist is in view');
      return;
    }
    change('/api/move', { id: row.trackId, dir, playlist: list });
    return;
  }
  const parent = rowByKey(lib, row.parent ?? null);
  if (row.kind !== 'track' || parent === undefined || !isAlbum(parent)) {
    refuse('Only tracks of an album can be moved');
    return;
  }
  if (isFiltering(filter)) {
    refuse('Clear the filter first, so the whole album is in view');
    return;
  }
  change('/api/move', { id: row.trackId, dir });
}

function renumber(): void {
  const at = cursorRow();
  if (at === undefined) {
    return;
  }
  const { lib } = at;
  if (isFlat(lib)) {
    refuse('Numbering an album needs the tree view · S, then Tree');
    return;
  }
  const album = at.row.kind === 'track' ? rowByKey(lib, at.row.parent ?? null) : at.row;
  if (album === undefined || !isAlbum(album)) {
    refuse('Select an album, or one of its tracks, to number it');
    return;
  }
  if (isFiltering(useApp.getState().filter)) {
    refuse('Clear the filter first, so the whole album is in view');
    return;
  }
  change('/api/renumber', { key: album.key });
}

function history(which: 'undo' | 'redo'): void {
  const step = useApp.getState().lib?.history[which];
  if (step === undefined || step === '') {
    setStatus(which === 'undo' ? 'Nothing to undo' : 'Nothing to redo');
    return;
  }
  change(`/api/${which}`);
}

async function clipboardPicture(): Promise<File | undefined> {
  try {
    for (const item of await navigator.clipboard.read()) {
      const type = item.types.find((t) => pictureType.test(t));
      if (type !== undefined) {
        return new File([await item.getType(type)], `clipboard.${type.slice('image/'.length)}`, { type });
      }
    }
  } catch {
    return undefined;
  }
  return undefined;
}

async function paste(): Promise<void> {
  const at = cursorRow();
  const { marks } = useApp.getState();
  const row = at?.row;
  const onTarget = row !== undefined && (row.kind === 'track' || row.kind === 'artist' || isAlbum(row));
  if (at === undefined || (marks.size === 0 && !onTarget)) {
    refuse('Select a track, an album or an artist to paste on');
    return;
  }
  const clip = at.lib.history.clip;
  if (clip === undefined || clip === '') {
    const picture = await clipboardPicture();
    if (picture !== undefined) {
      await pastePicture(picture);
      return;
    }
    refuse('Nothing is copied yet: c copies from a track · Ctrl+V pastes a picture');
    return;
  }
  let body: { ids: string[] } | { group: string };
  if (marks.size > 0) {
    body = { ids: markedIds(at.lib.view.rows, marks) };
  } else if (at.row.kind === 'track' && at.row.trackId !== undefined) {
    body = { ids: [at.row.trackId] };
  } else {
    body = { group: at.row.key };
  }
  await attempt(() => send<ChangeResponse>('POST', '/api/paste', body));
}

function copyField(): void {
  const s = useApp.getState();
  const f = currentField(s);
  if (isTrack(s.detail) && f !== undefined) {
    change('/api/copy', { id: s.detail.id, field: f.key });
  }
}

export function runOps(action: ActionName): boolean {
  switch (action) {
    case 'bulkEdit':
      bulkEdit();
      return true;
    case 'remove':
      remove();
      return true;
    case 'moveTrackDown':
      moveTrack(1);
      return true;
    case 'moveTrackUp':
      moveTrack(-1);
      return true;
    case 'renumber':
      renumber();
      return true;
    case 'undo':
      history('undo');
      return true;
    case 'redo':
      history('redo');
      return true;
    case 'paste':
      void paste();
      return true;
    case 'copy':
      copyField();
      return true;
    default:
      return false;
  }
}
