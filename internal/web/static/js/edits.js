import { S } from './state.js';
import { post, patch, put, del, query } from './api.js';
import { plural } from './dom.js';
import * as a from './actions.js';
import * as tree from './tree.js';
import * as inspector from './inspector.js';
import { closeOverlay } from './overlay.js';
import { ask, bulkForm, copyList, playlistPicker } from './dialogs.js';

// Everything that changes the library. The server does the changing and keeps
// the undo history; this file works out what the person pointed at, sends it,
// and shows how it went.

const enc = encodeURIComponent;

// ---- sending a change

// change sends one change and shows its outcome: the library is read again,
// the cursor goes to the track changed, and renamed rows keep their place.
// It resolves to the answer, or throws the reason it was refused.
async function change(call) {
  if (S.busy) throw new Error('');
  S.busy = true;
  try {
    const res = await call();
    S.history = res.history || S.history;
    if (res.message) a.setStatus(res.message);
    if (!res.same && (res.message || res.reveal)) await a.loadLibrary({ moves: res.moves, reveal: res.reveal });
    else if (res.same) a.renderHints();
    return res;
  } finally {
    S.busy = false;
  }
}

// attempt is change for a key that has no form to show a refusal in: the
// reason goes to the status line.
async function attempt(call) {
  try {
    return await change(call);
  } catch (err) {
    if (err.message) a.fail(err);
    return null;
  }
}

// ---- what a key is about

const row = () => tree.cursorRow();
const inFlatList = () => tree.flatList();

// targets are the tracks that v, p and a dropped picture are about: the marked
// ones, or what the cursor is on.
function targets() {
  if (S.marks.size) return tree.markedIds();
  const r = row();
  return r ? tree.idsOf(r) : [];
}

// playlistOf names the playlist a row is, or is a track in.
function playlistOf(r) {
  if (!r) return '';
  if (r.kind === 'playlist') return r.label;
  const parent = r.kind === 'track' && r.parent ? tree.rowByKey(r.parent) : null;
  return parent && parent.kind === 'playlist' ? parent.label : '';
}

const isKey = (r, prefix) => r && r.key.startsWith(prefix);

// ---- typing in a field

// startEdit opens the field under the cursor of the right pane for typing, as
// Enter does; a cover opens the file chooser instead.
export function startEdit() {
  const f = inspector.currentField();
  const d = S.detail;
  if (!f || !d || S.edit) return;
  if (f.key === 'wolfgang') return;
  if (!f.editable) return a.setStatus(`There is no ${f.label.toLowerCase()} to write in Latin letters`);
  if (f.key === 'cover') return chooseCover();
  const scope = d.kind === 'track' ? 'track' : 'group';
  S.edit = {
    scope, index: S.field, key: f.key, label: f.label, mixed: !!f.mixed,
    text: f.edit ?? f.text ?? '', origText: f.edit ?? f.text ?? '',
    hasSecond: scope === 'track' && f.key === 'track', second: false, secondText: f.second ?? '', origSecond: f.second ?? '',
    error: '', note: '', prefilled: !!f.dim && scope === 'track',
  };
  inspector.renderInspector();
  a.renderHints();
  previewNote();
}

export function cancelEdit() {
  if (!S.edit) return;
  S.edit = null;
  inspector.renderInspector();
  a.renderHints();
  document.getElementById('tree').focus({ preventScroll: true });
}

function typed(which, value) {
  const e = S.edit;
  if (!e) return;
  if (which === 'secondText') e.secondText = value;
  else e.text = value;
  e.error = '';
  const err = document.getElementById('edit-error');
  if (err) err.textContent = '';
  clearTimeout(noteTimer);
  noteTimer = setTimeout(previewNote, 150);
}

// previewNote shows what typing would do: where the file would go, or the
// name of the group the text would join.
let noteTimer = 0;
let noteSeq = 0;
async function previewNote() {
  const e = S.edit;
  if (!e) return;
  const mine = ++noteSeq;
  let note = '';
  try {
    if (e.scope === 'track' && ['title', 'artist', 'album', 'track'].includes(e.key)) {
      const res = await post(`/api/tracks/${enc(S.detail.data.id)}/path`, { field: e.key, text: e.text, second: e.secondText });
      note = `→ ${res.path}`;
    } else if (e.scope === 'group' && ['artist', 'album'].includes(e.key)) {
      note = (await post('/api/group/hint', { key: S.cursor, field: e.key, text: e.text })).hint;
    }
  } catch { /* the note is a courtesy: saving says what is wrong */ }
  if (mine !== noteSeq || S.edit !== e) return;
  e.note = note;
  const el = document.getElementById('edit-note');
  if (el) el.textContent = note;
}

// saveEdit sends the text typed; a refusal is shown under the field.
export async function saveEdit() {
  const e = S.edit;
  if (!e) return;
  const unchanged = e.text === e.origText && (!e.hasSecond || e.secondText === e.origSecond);
  if (unchanged && !e.prefilled) return cancelEdit();
  try {
    if (e.scope === 'track') {
      await change(() => patch(`/api/tracks/${enc(S.detail.data.id)}`, { field: e.key, text: e.text, second: e.secondText }));
    } else {
      await change(() => patch('/api/group', { key: S.cursor, field: e.key, text: e.text }));
    }
  } catch (err) {
    if (!err.message || S.edit !== e) return;
    e.error = err.message;
    const el = document.getElementById('edit-error');
    if (el) el.textContent = err.message;
    return;
  }
  S.edit = null;
  inspector.renderInspector();
  a.renderHints();
  document.getElementById('tree').focus({ preventScroll: true });
}

a.hooks.editField = startEdit;
a.hooks.stopPlaying = stopPlaying;
inspector.hooks.save = saveEdit;
inspector.hooks.cancel = cancelEdit;
inspector.hooks.typed = typed;

// ---- covers

// coverTarget says what a cover is for: the track in the pane, the group in the
// pane, or, with tracks marked, those.
function coverTarget(fromPane) {
  if (!fromPane && S.marks.size) return { ids: tree.markedIds() };
  const r = row();
  if (!r) return null;
  if (r.kind === 'track') return { ids: [r.trackId] };
  if (r.kind === 'artist' || (r.kind === 'album' && !isKey(r, 'singles'))) return { group: r.key };
  return null;
}

const coverQuery = (t, extra) => query({ ...extra, ids: t.ids ? t.ids.join(',') : '', group: t.group || '' });

let chooser = null;
// chooseCover opens the file chooser; the file picked becomes the cover of what the pane shows.
export function chooseCover() {
  if (!chooser) {
    chooser = document.createElement('input');
    chooser.type = 'file';
    chooser.accept = 'image/jpeg,image/png';
    chooser.addEventListener('change', () => {
      const file = chooser.files[0];
      chooser.value = '';
      if (file) setCover(file, coverTarget(true));
    });
  }
  chooser.click();
}

export async function setCover(file, target, { paste = false } = {}) {
  if (!target) return a.setStatus('Select a track, an album or an artist for the cover', 'bad');
  if (!/^image\/(jpeg|png)$/.test(file.type)) return a.setStatus('A cover must be a JPEG or PNG image', 'bad');
  await attempt(() => put(`/api/cover${coverQuery(target, paste ? { paste: 1, name: file.name } : {})}`, file));
}

export async function removeCover() {
  const f = inspector.currentField();
  if (!f || f.key !== 'cover') return;
  const target = coverTarget(true);
  if (!target) return;
  const had = S.detail.kind === 'track' ? !!S.detail.data.coverSha : !!S.detail.data.coverTrack;
  if (!had) return a.setStatus('There is no cover to remove');
  await attempt(() => del(`/api/cover${coverQuery(target)}`));
}

// pasted and dropped pictures
export function pictureIn(files) {
  return [...(files || [])].find((f) => /^image\/(jpeg|png)$/.test(f.type));
}
export function pastePicture(file) {
  return setCover(file, coverTarget(false), { paste: true });
}
export function dropPicture(file) {
  return setCover(file, coverTarget(false));
}

// ---- nudging, clearing, hearing

const NUDGEABLE = ['cutStart', 'cutEnd', 'volume', 'fadeIn', 'fadeOut'];
const field = () => {
  const f = inspector.currentField();
  return f && S.detail && S.detail.kind === 'track' ? f : null;
};

export async function nudge(more, big) {
  const f = field();
  if (!f || !NUDGEABLE.includes(f.key)) return;
  await attempt(() => post(`/api/tracks/${enc(S.detail.data.id)}/nudge`, { field: f.key, more, big }));
}

export async function clearField() {
  const f = field();
  if (!f || !NUDGEABLE.includes(f.key)) return;
  await attempt(() => post(`/api/tracks/${enc(S.detail.data.id)}/clear`, { field: f.key }));
}

let audio = null;
export function stopPlaying() {
  if (!audio) return false;
  audio.pause();
  URL.revokeObjectURL(audio.src);
  audio = null;
  S.playing = null;
  a.clearStatus();
  a.renderHints();
  return true;
}

// play sounds the audio around the cut, fade or volume under the cursor, or stops it.
export async function play() {
  const f = field();
  if (!f || !NUDGEABLE.includes(f.key)) return;
  if (stopPlaying()) return a.setStatus('');
  const id = S.detail.data.id;
  a.setStatus('Cutting the preview…');
  S.playing = { id, field: f.key, loading: true };
  try {
    const res = await fetch(`/api/tracks/${enc(id)}/preview${query({ field: f.key })}`);
    if (!res.ok) {
      const data = await res.json().catch(() => ({}));
      throw new Error(data.error || `${res.status} ${res.statusText}`);
    }
    const blob = await res.blob();
    if (!S.playing || S.playing.id !== id) return; // stopped while it was being cut
    audio = new Audio(URL.createObjectURL(blob));
    audio.addEventListener('ended', () => { stopPlaying(); a.setStatus(''); });
    S.playing.loading = false;
    await audio.play();
    a.setStatus('Playing · p or esc stops');
    a.renderHints();
  } catch (err) {
    stopPlaying();
    a.fail(err);
  }
}

// ---- copy and paste

export async function copyKey() {
  const r = row();
  if (S.focus === 'inspector') {
    const f = field();
    if (!f) return;
    await attempt(() => post('/api/copy', { id: S.detail.data.id, field: f.key }));
    return;
  }
  if (!r || r.kind !== 'track') return a.setStatus('Select a track to copy from', 'bad');
  try {
    await copyList({
      id: r.trackId, title: r.label,
      onCopy: async (attrs) => {
        try {
          await change(() => post('/api/copy', { id: r.trackId, attrs }));
          closeOverlay();
          return '';
        } catch (err) {
          return err.message;
        }
      },
    });
  } catch (err) {
    a.fail(err);
  }
}

export async function paste() {
  const r = row();
  const ok = S.marks.size || (r && (r.kind === 'track' || r.kind === 'artist' || (r.kind === 'album' && !isKey(r, 'singles'))));
  if (!ok) return a.setStatus('Select a track, an album or an artist to paste on', 'bad');
  if (!S.history.clip) {
    // nothing copied inside: a picture on the desktop clipboard may be there
    const file = await clipboardPicture();
    if (file) return pastePicture(file);
    return a.setStatus('Nothing is copied yet: c copies from a track · Ctrl+V pastes a picture', 'bad');
  }
  const body = S.marks.size ? { ids: tree.markedIds() } : r.kind === 'track' ? { ids: [r.trackId] } : { group: r.key };
  await attempt(() => post('/api/paste', body));
}

async function clipboardPicture() {
  try {
    for (const item of await navigator.clipboard.read()) {
      const type = item.types.find((t) => /^image\/(jpeg|png)$/.test(t));
      if (type) return new File([await item.getType(type)], 'clipboard.' + type.split('/')[1], { type });
    }
  } catch { /* the browser did not allow it, or there is no picture */ }
  return null;
}

// ---- removing, moving, numbering

export function remove() {
  const r = row();
  if (!r) return;
  const list = playlistOf(r);
  if (list) return removeInPlaylist(r, list);
  if (r.key === 'playlists') return a.setStatus('Select a playlist, or one of its tracks, to take out of it', 'bad');
  const names = tree.titles();
  let ids;
  let head;
  if (S.marks.size) {
    ids = tree.markedIds();
    head = `Remove ${plural(ids.length, 'marked track')}?`;
  } else {
    ids = tree.idsOf(r);
    if (r.kind === 'track') head = `Remove “${r.label}”?`;
    else if (r.kind === 'artist') head = `Remove “${r.label}” and its ${plural(ids.length, 'track')}?`;
    else if (r.kind === 'album') head = `Remove the ${plural(ids.length, 'track')} of “${r.label}”?`;
    else head = `Remove everything in “${r.label}” (${plural(ids.length, 'track')})?`;
  }
  if (ids.length === 0) return;
  const body = [];
  if (ids.length > 1) {
    ids.slice(0, 5).forEach((id) => body.push(`• ${names.get(id) || id}`));
    if (ids.length > 5) body.push(`…and ${ids.length - 5} more`);
  }
  body.push(`Nothing is deleted from disk until you sync with prune, and u brings ${ids.length > 1 ? 'them' : 'it'} back until then.`);
  ask({ head, body, yes: 'remove', onYes: () => attempt(() => post('/api/remove', { ids })) });
}

function removeInPlaylist(r, list) {
  if (r.kind === 'playlist') {
    return ask({
      head: `Delete the playlist “${list}”?`,
      body: [`Its ${plural(r.count, 'track')} stay in the library.`, 'u brings the playlist back.'],
      yes: 'delete', onYes: () => attempt(() => post('/api/playlists/delete', { name: list })),
    });
  }
  const parent = tree.rowByKey(r.parent);
  let ids = tree.idsOf(parent).filter((id) => S.marks.has(id));
  if (ids.length === 0) ids = [r.trackId];
  attempt(() => post('/api/playlists/takeout', { name: list, ids }));
}

export async function moveTrack(dir) {
  const r = row();
  if (!r) return;
  if (inFlatList()) return a.setStatus('Moving a track needs the tree view · S, then Tree', 'bad');
  const list = r.kind === 'track' ? playlistOf(r) : '';
  if (list) {
    if (S.filter.trim()) return a.setStatus('Clear the filter first, so the whole playlist is in view', 'bad');
    return void attempt(() => post('/api/move', { id: r.trackId, dir, playlist: list }));
  }
  if (r.kind !== 'track' || !isKey(tree.rowByKey(r.parent), 'album')) return a.setStatus('Only tracks of an album can be moved', 'bad');
  if (S.filter.trim()) return a.setStatus('Clear the filter first, so the whole album is in view', 'bad');
  await attempt(() => post('/api/move', { id: r.trackId, dir }));
}

export async function renumber() {
  if (inFlatList()) return a.setStatus('Numbering an album needs the tree view · S, then Tree', 'bad');
  let r = row();
  if (r && r.kind === 'track' && r.parent) r = tree.rowByKey(r.parent);
  if (!r || r.kind !== 'album' || isKey(r, 'singles')) return a.setStatus('Select an album, or one of its tracks, to number it', 'bad');
  if (S.filter.trim()) return a.setStatus('Clear the filter first, so the whole album is in view', 'bad');
  await attempt(() => post('/api/renumber', { key: r.key }));
}

// ---- playlists

export function playlistKey() {
  const r = row();
  let ids = [];
  if (S.marks.size) ids = tree.markedIds();
  else if (r && r.key !== 'playlists') ids = tree.idsOf(r);
  if (ids.length === 0) return a.setStatus('Select tracks to add to a playlist: a track, an album, or mark several with space', 'bad');
  playlistPicker({
    count: ids.length,
    choose: async (name, isNew) => {
      try {
        await change(() => post(isNew ? '/api/playlists' : '/api/playlists/add', { name, ids }));
        closeOverlay();
        return '';
      } catch (err) {
        return err.message;
      }
    },
  });
}

// ---- several tracks

// bulkEdit is e in the tree: the form for the marked tracks, or, on what has
// fields of its own, the fields.
export function bulkEdit() {
  const r = row();
  if (!r) return;
  if (r.kind === 'playlist') return void a.focusInspector();
  if (r.key === 'playlists') return a.setStatus('Select a playlist to edit it', 'bad');
  let ids;
  let head;
  if (S.marks.size) {
    ids = tree.markedIds();
    head = `Edit ${plural(ids.length, 'marked track')}`;
  } else if (a.focusInspector()) {
    return;
  } else {
    ids = tree.idsOf(r);
    head = `Edit ${plural(ids.length, 'track')}`;
  }
  if (ids.length === 0) return;
  bulkForm({
    head, ids,
    preview: (tracks, values) => post('/api/bulk/preview', { ids: tracks, values }),
    apply: async (tracks, values) => {
      try {
        await change(() => post('/api/bulk', { ids: tracks, values }));
        closeOverlay();
        return '';
      } catch (err) {
        return err.message;
      }
    },
  });
}

// ---- undo

export async function undo() {
  if (!S.history.undo) return a.setStatus('Nothing to undo');
  await attempt(() => post('/api/undo'));
}
export async function redo() {
  if (!S.history.redo) return a.setStatus('Nothing to redo');
  await attempt(() => post('/api/redo'));
}

