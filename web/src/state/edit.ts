import { ApiError, getBlob, query, send } from '../api/client';
import type { ChangeResponse, FieldView, HintView, PathView } from '../api/types.gen';
import { dropDetail, loadDetail } from './detail';
import { loadLibrary } from './library';
import { pointRow } from './actions';
import { fail, setStatus } from './status';
import { chooseCover, removeCover } from './cover';
import { currentField, fieldIndex, fieldList, isTrack, nudgeable } from './inspector';
import { useApp, type EditState } from './store';
import { ancestorKeys, rowByKey } from './tree';

const notePause = 150;
const pathFields = ['title', 'artist', 'album', 'track'];
const joinFields = ['artist', 'album'];

export async function applyChange(call: () => Promise<ChangeResponse>): Promise<ChangeResponse> {
  if (useApp.getState().busy) {
    throw new Error('');
  }
  useApp.setState({ busy: true });
  const dropped = dropDetail();
  let reloaded = false;
  try {
    const res = await call();
    if (res.message !== undefined && res.message !== '') {
      setStatus(res.message);
    }
    if (res.same !== true && (res.message !== undefined || res.reveal !== undefined)) {
      reloaded = true;
      await loadLibrary({ moves: res.moves, reveal: res.reveal });
    }
    return res;
  } finally {
    useApp.setState({ busy: false });
    if (dropped && !reloaded) {
      void loadDetail();
    }
  }
}

export async function attempt(call: () => Promise<ChangeResponse>): Promise<void> {
  try {
    await applyChange(call);
  } catch (err) {
    if (err instanceof Error && err.message !== '') {
      fail(err);
    }
  }
}

function editedField(): FieldView | undefined {
  const s = useApp.getState();
  return s.edit === null ? undefined : fieldList(s)[s.edit.field];
}

function updateEdit(patch: Partial<EditState>): void {
  const { edit } = useApp.getState();
  if (edit !== null) {
    useApp.setState({ edit: { ...edit, ...patch } });
  }
}

let noteTimer: ReturnType<typeof setTimeout> | undefined;
let noteSeq = 0;

async function noteFor(f: FieldView, edit: EditState): Promise<string> {
  const { detail, cursor } = useApp.getState();
  if (isTrack(detail) && pathFields.includes(f.key)) {
    const res = await send<PathView>('POST', `/api/tracks/${encodeURIComponent(detail.id)}/path`, { field: f.key, text: edit.text, second: edit.second ?? '' });
    return `→ ${res.path}`;
  }
  if (detail !== null && !isTrack(detail) && joinFields.includes(f.key)) {
    return (await send<HintView>('POST', '/api/group/hint', { key: cursor, field: f.key, text: edit.text })).hint;
  }
  return '';
}

async function showNote(): Promise<void> {
  const { edit } = useApp.getState();
  const f = editedField();
  if (edit === null || f === undefined) {
    return;
  }
  noteSeq += 1;
  const mine = noteSeq;
  const note = await noteFor(f, edit).catch(() => '');
  if (mine === noteSeq) {
    updateEdit({ note });
  }
}

export function startEdit(): void {
  const s = useApp.getState();
  const f = currentField(s);
  if (f === undefined || s.edit !== null) {
    return;
  }
  if (!f.editable) {
    setStatus(`There is no ${f.label.toLowerCase()} to write in Latin letters`);
    return;
  }
  if (f.key === 'cover') {
    chooseCover();
    return;
  }
  useApp.setState({ edit: { field: fieldIndex(s), text: f.edit, second: f.second ?? '' } });
  void showNote();
}

export function typeEdit(which: 'text' | 'second', text: string): void {
  updateEdit(which === 'text' ? { text, error: '', wrong: {} } : { second: text, error: '', wrong: {} });
  clearTimeout(noteTimer);
  noteTimer = setTimeout(() => {
    void showNote();
  }, notePause);
}

export function cancelEdit(): void {
  clearTimeout(noteTimer);
  noteSeq += 1;
  useApp.setState({ edit: null });
}

function saveCall(f: FieldView, edit: EditState): () => Promise<ChangeResponse> {
  const { detail, cursor } = useApp.getState();
  if (isTrack(detail)) {
    return () => send<ChangeResponse>('PATCH', `/api/tracks/${encodeURIComponent(detail.id)}`, { field: f.key, text: edit.text, second: edit.second ?? '' });
  }
  return () => send<ChangeResponse>('PATCH', '/api/group', { key: cursor, field: f.key, text: edit.text });
}

export async function saveEdit(): Promise<void> {
  const { edit, detail } = useApp.getState();
  const f = editedField();
  if (edit === null || f === undefined) {
    return;
  }
  const unchanged = edit.text === f.edit && (f.key !== 'track' || (edit.second ?? '') === (f.second ?? ''));
  const prefilled = f.dim === true && isTrack(detail);
  if (unchanged && !prefilled) {
    cancelEdit();
    return;
  }
  try {
    await applyChange(saveCall(f, edit));
  } catch (err) {
    if (err instanceof ApiError && Object.keys(err.fields).length > 0) {
      updateEdit({ wrong: err.fields });
    } else if (err instanceof Error && err.message !== '') {
      updateEdit({ error: err.message });
    }
    return;
  }
  cancelEdit();
}

export function chooseField(i: number): void {
  const s = useApp.getState();
  const again = s.focus === 'inspector' && fieldIndex(s) === i && s.edit === null;
  useApp.setState({ focus: 'inspector', field: i });
  if (again) {
    startEdit();
  }
}

function cutField(): { id: string; key: string } | undefined {
  const s = useApp.getState();
  const f = currentField(s);
  if (!isTrack(s.detail) || f === undefined || !nudgeable.includes(f.key)) {
    return undefined;
  }
  return { id: s.detail.id, key: f.key };
}

export function nudge(more: 1 | -1, big: boolean): void {
  const cut = cutField();
  if (cut !== undefined) {
    void attempt(() => send<ChangeResponse>('POST', `/api/tracks/${encodeURIComponent(cut.id)}/nudge`, { field: cut.key, more, big }));
  }
}

export function clear(): void {
  if (currentField(useApp.getState())?.key === 'cover') {
    removeCover();
    return;
  }
  const cut = cutField();
  if (cut !== undefined) {
    void attempt(() => send<ChangeResponse>('POST', `/api/tracks/${encodeURIComponent(cut.id)}/clear`, { field: cut.key }));
  }
}

export function stopPlaying(): boolean {
  if (useApp.getState().playing === null) {
    return false;
  }
  useApp.setState({ playing: null });
  setStatus('');
  return true;
}

export async function hear(): Promise<void> {
  const cut = cutField();
  if (cut === undefined || stopPlaying()) {
    return;
  }
  const playing = { id: cut.id, field: cut.key };
  useApp.setState({ playing });
  setStatus('Cutting the preview…');
  try {
    const blob = await getBlob(`/api/tracks/${encodeURIComponent(cut.id)}/preview${query({ field: cut.key })}`);
    if (useApp.getState().playing !== playing) {
      return;
    }
    useApp.setState({ playing: { ...playing, src: URL.createObjectURL(blob) } });
    setStatus('Playing · space or esc stops');
  } catch (err) {
    if (useApp.getState().playing === playing) {
      stopPlaying();
      fail(err);
    }
  }
}

export function goToRow(key: string): void {
  const { lib, expanded } = useApp.getState();
  const row = rowByKey(lib, key);
  if (lib === null || row === undefined) {
    return;
  }
  useApp.setState({ expanded: new Set([...expanded, ...ancestorKeys(lib.view.rows, row)]) });
  pointRow(key);
}

useApp.subscribe((state, previous) => {
  const src = previous.playing?.src;
  if (src !== undefined && state.playing?.src !== src) {
    URL.revokeObjectURL(src);
  }
});
