import { getBlob, query, send } from '../api/client';
import type { ChangeResponse, FieldView, HintView, PathView } from '../api/types.gen';
import type { ActionName } from '../keys/keymap';
import { fail, loadLibrary, pointRow, setStatus } from './actions';
import { chooseTab, currentField, fieldIndex, fieldList, isTrack, moveField, nextTab, nudgeable, previousTabOrBack } from './inspector';
import { useApp, type EditState } from './store';
import { ancestorKeys, rowByKey } from './tree';

const notePause = 150;
const pathFields = ['title', 'artist', 'album', 'track'];
const joinFields = ['artist', 'album'];
const enc = encodeURIComponent;

export async function applyChange(call: () => Promise<ChangeResponse>): Promise<ChangeResponse> {
  if (useApp.getState().busy) {
    throw new Error('');
  }
  useApp.setState({ busy: true });
  try {
    const res = await call();
    if (res.message !== undefined && res.message !== '') {
      setStatus(res.message);
    }
    if (res.same !== true && (res.message !== undefined || res.reveal !== undefined)) {
      await loadLibrary({ moves: res.moves, reveal: res.reveal });
    }
    return res;
  } finally {
    useApp.setState({ busy: false });
  }
}

async function attempt(call: () => Promise<ChangeResponse>): Promise<void> {
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
    const res = await send<PathView>('POST', `/api/tracks/${enc(detail.id)}/path`, { field: f.key, text: edit.text, second: edit.second ?? '' });
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
  if (f === undefined || s.edit !== null || f.key === 'cover') {
    return;
  }
  if (!f.editable) {
    setStatus(`There is no ${f.label.toLowerCase()} to write in Latin letters`);
    return;
  }
  useApp.setState({ edit: { field: fieldIndex(s), text: f.edit, second: f.second ?? '' } });
  void showNote();
}

export function typeEdit(which: 'text' | 'second', text: string): void {
  updateEdit(which === 'text' ? { text, error: '' } : { second: text, error: '' });
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
    return () => send<ChangeResponse>('PATCH', `/api/tracks/${enc(detail.id)}`, { field: f.key, text: edit.text, second: edit.second ?? '' });
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
    if (err instanceof Error && err.message !== '') {
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

function nudge(more: 1 | -1, big: boolean): void {
  const cut = cutField();
  if (cut !== undefined) {
    void attempt(() => send<ChangeResponse>('POST', `/api/tracks/${enc(cut.id)}/nudge`, { field: cut.key, more, big }));
  }
}

function clearField(): void {
  const cut = cutField();
  if (cut !== undefined) {
    void attempt(() => send<ChangeResponse>('POST', `/api/tracks/${enc(cut.id)}/clear`, { field: cut.key }));
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

async function hear(): Promise<void> {
  const cut = cutField();
  if (cut === undefined || stopPlaying()) {
    return;
  }
  const playing = { id: cut.id, field: cut.key };
  useApp.setState({ playing });
  setStatus('Cutting the preview…');
  try {
    const blob = await getBlob(`/api/tracks/${enc(cut.id)}/preview${query({ field: cut.key })}`);
    if (useApp.getState().playing !== playing) {
      return;
    }
    useApp.setState({ playing: { ...playing, src: URL.createObjectURL(blob) } });
    setStatus('Playing · p or esc stops');
  } catch (err) {
    if (useApp.getState().playing === playing) {
      stopPlaying();
      fail(err);
    }
  }
}

function removeCover(): void {
  const s = useApp.getState();
  const d = s.detail;
  if (currentField(s)?.key !== 'cover' || d === null) {
    return;
  }
  const had = isTrack(d) ? (d.coverSha ?? '') !== '' : (d.coverTrack ?? '') !== '';
  if (!had) {
    setStatus('There is no cover to remove');
    return;
  }
  const target = isTrack(d) ? { ids: d.id } : { group: s.cursor };
  void attempt(() => send<ChangeResponse>('DELETE', '/api/cover' + query(target)));
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

const tabKeys: Partial<Record<ActionName, number>> = { tab1: 0, tab2: 1, tab3: 2, tab4: 3 };

export function runInspector(action: ActionName): void {
  const tab = tabKeys[action];
  if (tab !== undefined) {
    chooseTab(tab);
    return;
  }
  switch (action) {
    case 'fieldUp':
      moveField(-1);
      break;
    case 'fieldDown':
      moveField(1);
      break;
    case 'nextTab':
      nextTab();
      break;
    case 'previousTabOrBack':
      previousTabOrBack();
      break;
    case 'startEdit':
      startEdit();
      break;
    case 'nudgeDown':
      nudge(-1, false);
      break;
    case 'nudgeUp':
      nudge(1, false);
      break;
    case 'nudgeDownMore':
      nudge(-1, true);
      break;
    case 'nudgeUpMore':
      nudge(1, true);
      break;
    case 'clearField':
      clearField();
      break;
    case 'hear':
      void hear();
      break;
    case 'removeCover':
      removeCover();
      break;
    default:
      break;
  }
}
