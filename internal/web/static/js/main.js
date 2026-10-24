import { S, safeGet, safeSet } from './state.js';
import { get } from './api.js';
import * as a from './actions.js';
import * as tree from './tree.js';
import { onKeydown } from './keys.js';
import * as edits from './edits.js';

const $ = (id) => document.getElementById(id);

async function boot() {
  try {
    S.info = await get('/api/info');
  } catch (err) {
    a.fail(err);
    return;
  }
  document.addEventListener('keydown', onKeydown);
  document.addEventListener('mm:hints', () => { a.renderHints(); a.renderFocus(); });
  $('filter').addEventListener('input', a.typedFilter);
  $('filter').addEventListener('focus', a.renderHints);
  $('filter').addEventListener('blur', a.renderHints);
  wireTheme();
  $('sync-chip').addEventListener('click', () => a.notYet('Sync'));
  wireTree();
  wireSplit();
  wirePictures();
  await a.loadLibrary();
  a.renderFocus();
  a.renderHints();
  listen();
  $('tree').focus({ preventScroll: true });
}

// The page is dark, like the terminal it stands in for; the light one is a click away.
function wireTheme() {
  const apply = (t) => { if (t === 'light') document.documentElement.dataset.theme = 'light'; else delete document.documentElement.dataset.theme; };
  apply(safeGet('mm.theme'));
  $('theme').addEventListener('click', () => {
    const next = document.documentElement.dataset.theme === 'light' ? 'dark' : 'light';
    apply(next);
    safeSet('mm.theme', next);
  });
}

// A click on a row puts the cursor there; a second one opens it, or goes into
// a track's fields. A click on the arrow opens or closes at once.
function wireTree() {
  const el = $('tree');
  el.addEventListener('click', (e) => {
    const row = e.target.closest('.row');
    if (!row) return;
    const r = tree.rowByKey(row.dataset.key);
    if (!r) return;
    if (S.edit) edits.cancelEdit();
    a.focusTree();
    if (e.target.classList.contains('twist')) {
      tree.setCursor(r.key);
      tree.toggle(r);
    } else if (e.target.classList.contains('mark')) {
      tree.setCursor(r.key);
      a.toggleMark();
    } else {
      tree.setCursor(r.key);
    }
  });
  el.addEventListener('dblclick', (e) => {
    const row = e.target.closest('.row');
    if (!row) return;
    const r = tree.rowByKey(row.dataset.key);
    if (r && !e.target.classList.contains('twist')) a.enter();
  });
  el.addEventListener('focus', () => { if (S.focus !== 'tree') a.focusTree(); });
}

// A picture pasted with Ctrl+V or dropped on the page becomes the cover of the
// marked tracks, or of what the cursor is on.
function wirePictures() {
  const typing = () => /^(INPUT|TEXTAREA|SELECT)$/.test((document.activeElement || {}).tagName || '');
  document.addEventListener('paste', (ev) => {
    if (S.overlay || typing()) return;
    const file = edits.pictureIn(ev.clipboardData && ev.clipboardData.files);
    if (!file) return;
    ev.preventDefault();
    edits.pastePicture(file);
  });
  const carries = (ev) => ev.dataTransfer && [...ev.dataTransfer.types].includes('Files');
  const body = document.body;
  document.addEventListener('dragover', (ev) => {
    if (!carries(ev) || S.overlay) return;
    ev.preventDefault();
    body.classList.add('dropping');
  });
  document.addEventListener('dragleave', (ev) => {
    if (ev.relatedTarget === null) body.classList.remove('dropping');
  });
  document.addEventListener('drop', (ev) => {
    if (!carries(ev)) return;
    ev.preventDefault();
    body.classList.remove('dropping');
    if (S.overlay) return;
    const file = edits.pictureIn(ev.dataTransfer.files);
    if (file) edits.dropPicture(file);
    else a.setStatus('Only a JPEG or PNG picture can be dropped here: it becomes a cover', 'bad');
  });
}

// The line between the panes can be dragged; where it was left is remembered.
function wireSplit() {
  const saved = safeGet('mm.left');
  if (saved) $('main').style.setProperty('--left', saved);
  const split = $('split');
  split.addEventListener('pointerdown', (e) => {
    split.setPointerCapture(e.pointerId);
    split.classList.add('drag');
    const main = $('main');
    const move = (ev) => {
      const box = main.getBoundingClientRect();
      const px = Math.min(Math.max(ev.clientX - box.left, 220), box.width - 300);
      main.style.setProperty('--left', `${px}px`);
    };
    const up = () => {
      split.classList.remove('drag');
      split.removeEventListener('pointermove', move);
      split.removeEventListener('pointerup', up);
      safeSet('mm.left', main.style.getPropertyValue('--left'));
    };
    split.addEventListener('pointermove', move);
    split.addEventListener('pointerup', up);
  });
}

// The server tells the page when the library changed, whoever changed it.
function listen() {
  const es = new EventSource('/api/events');
  es.addEventListener('library', (e) => {
    let stamp = '';
    try { stamp = JSON.parse(e.data); } catch { /* ask anyway */ }
    if (stamp !== S.stamp && !S.busy) a.loadLibrary();
  });
  es.addEventListener('open', () => {
    // after a reconnect the library may have changed meanwhile
    if (S.lib) a.loadLibrary();
  });
}

boot();
