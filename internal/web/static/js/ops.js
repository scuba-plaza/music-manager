import { S } from './state.js';
import { get, post } from './api.js';
import { h, name, plural, progress } from './dom.js';
import * as a from './actions.js';
import * as tree from './tree.js';
import { openOverlay, closeOverlay } from './overlay.js';
import { ask } from './dialogs.js';
import { change, attempt, dropPicture, pictureIn } from './edits.js';
import { follow, stopJob } from './jobs.js';

// What is done with the library as a whole rather than with a field: looking
// tracks up, adding new ones, checking that nothing is damaged.

const enc = encodeURIComponent;
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const moveIn = (n, i, d) => Math.min(Math.max(i + d, 0), n - 1);

function needLookup() {
  if (S.info && S.info.lookup) return true;
  a.setStatus('Lookup is not available: no MusicBrainz client is set up', 'bad');
  return false;
}

// ---- identify one track: choose what it is on MusicBrainz

export function identify() {
  const r = tree.cursorRow();
  if (!r || r.kind !== 'track' || !needLookup()) return;
  const id = r.trackId;
  let state = 'searching'; // searching, listing, failed, editing, applying
  let words = { title: r.label, artist: '' };
  let cands = [];
  let cursor = 0;
  let found = '';
  let error = '';
  let ctl = null;
  let inputs = [];
  let focus = 0;

  async function run(q) {
    if (ctl) ctl.abort();
    const mine = (ctl = new AbortController());
    state = 'searching';
    error = '';
    if (q) words = { title: q.title.trim(), artist: q.artist.trim() };
    overlay.redraw();
    try {
      const v = await post(`/api/tracks/${enc(id)}/candidates`, q ? { query: q } : {}, { signal: mine.signal });
      if (ctl !== mine) return;
      words = { title: v.title, artist: v.artist };
      cands = v.candidates;
      found = v.search;
      cursor = 0;
      state = 'listing';
    } catch (err) {
      if (err.name === 'AbortError' || ctl !== mine) return;
      error = err.message;
      state = 'failed';
    }
    overlay.redraw();
  }

  async function apply() {
    if (ctl) ctl.abort();
    const mine = (ctl = new AbortController());
    state = 'applying';
    overlay.redraw();
    try {
      const res = await change(() => post('/api/identify/apply', { search: found, index: cursor }, { signal: mine.signal }));
      if (ctl === mine) closeOverlay();
      return res;
    } catch (err) {
      if (err.name === 'AbortError') return;
      if (ctl !== mine) return;
      error = err.message;
      state = 'failed';
      overlay.redraw();
    }
  }

  // esc while the cover is being fetched: the track is identified without it
  async function skipCover() {
    if (ctl) ctl.abort();
    ctl = null;
    closeOverlay();
    a.setStatus('Identified without waiting for the cover');
    await sleep(600);
    a.loadLibrary({ reveal: id });
  }

  const quote = () => `“${words.title} – ${words.artist}”`;
  const overlay = {
    title: 'Identify',
    render(body) {
      if (state === 'searching') {
        body.append(h('div', { class: 'busy' }, `Searching MusicBrainz for ${quote()}…`), h('div', { class: 'ask-body' }, 'esc cancels'));
      } else if (state === 'applying') {
        body.append(h('div', { class: 'busy' }, 'Applying…'), h('div', { class: 'ask-body' }, 'Fetching the cover can take a moment; esc skips it.'));
      } else if (state === 'failed') {
        body.append(h('div', { class: 'warn' }, error), h('div', { class: 'ask-body' }, 'Press / to search with other words.'));
      } else if (state === 'editing') {
        body.append(h('div', { class: 'ask-body' }, 'Search for:'));
        inputs = ['title', 'artist'].map((k) => {
          const input = h('input', { class: 'edit', type: 'text', dir: 'auto', autocomplete: 'off', 'aria-label': k });
          input.value = words[k];
          return input;
        });
        body.append(h('div', { class: 'form' }, h('label', { class: 'label' }, 'Title', inputs[0]), h('label', { class: 'label' }, 'Artist', inputs[1])));
      } else {
        body.append(h('div', { class: 'ask-body' }, `${plural(cands.length, 'match')} for ${quote()}`));
        const list = h('div', { class: 'menu wide', role: 'listbox' });
        cands.forEach((c, i) => {
          list.append(h('div', { class: `item${i === cursor ? ' on' : ''}`, role: 'option', 'aria-selected': String(i === cursor), onclick: () => { cursor = i; overlay.redraw(); }, ondblclick: apply },
            h('span', { class: 'n' }), h('span', { class: 't' }, name(c.text)), h('span', { class: 'd' })));
        });
        body.append(list, h('div', { class: 'ask-body' }, 'This would change:'));
        const diff = h('div', { class: 'diff' });
        for (const d of cands[cursor].diff) {
          diff.append(h('span', { class: 'dim' }, d.label),
            d.same ? h('span', { class: 'dim' }, name(d.new), '  (same)') : h('span', {}, name(d.old), h('span', { class: 'dim' }, ' → '), h('span', { class: 'accent' }, name(d.new))));
        }
        body.append(diff);
      }
    },
    afterDraw() { if (state === 'editing' && inputs[focus]) inputs[focus].focus(); },
    onKey(e) {
      if (state === 'searching') {
        if (e.key === 'Escape') { if (ctl) ctl.abort(); closeOverlay(); } else return false;
      } else if (state === 'applying') {
        if (e.key === 'Escape') skipCover(); else return false;
      } else if (state === 'listing') {
        if (e.key === 'Escape' || e.key === 'q') { closeOverlay(); } else if (e.key === 'ArrowUp' || e.key === 'k') { cursor = moveIn(cands.length, cursor, -1); overlay.redraw(); }
        else if (e.key === 'ArrowDown' || e.key === 'j') { cursor = moveIn(cands.length, cursor, 1); overlay.redraw(); }
        else if (e.key === 'Home' || e.key === 'PageUp') { cursor = 0; overlay.redraw(); }
        else if (e.key === 'End' || e.key === 'PageDown') { cursor = cands.length - 1; overlay.redraw(); }
        else if (e.key === 'Enter') apply();
        else if (e.key === '/') { state = 'editing'; focus = 0; overlay.redraw(); }
        else return false;
      } else if (state === 'failed') {
        if (e.key === 'Escape' || e.key === 'q') closeOverlay();
        else if (e.key === '/') { state = 'editing'; focus = 0; overlay.redraw(); }
        else return false;
      } else if (state === 'editing') {
        if (e.key === 'Escape') { state = cands.length ? 'listing' : 'failed'; overlay.redraw(); }
        else if (e.key === 'Enter') run({ title: inputs[0].value, artist: inputs[1].value });
        else if (e.key === 'Tab' || e.key === 'ArrowDown' || e.key === 'ArrowUp') { focus = 1 - focus; inputs[focus].focus(); }
        else return false;
      }
      return true;
    },
    onClose() { if (ctl) ctl.abort(); },
    hints() {
      if (state === 'listing') return [['↑↓', 'choose'], ['enter', 'apply'], ['/', 'other words'], ['esc', 'close']];
      if (state === 'failed') return [['/', 'other words'], ['esc', 'close']];
      if (state === 'editing') return [['enter', 'search'], ['tab', 'next'], ['esc', 'back']];
      if (state === 'applying') return [['esc', 'skip the cover']];
      return [['esc', 'cancel']];
    },
  };
  openOverlay(overlay);
  run(null);
}

// ---- identify everything that needs work

export function identifyAll() {
  if (!needLookup()) return;
  const need = S.lib ? S.lib.needWork : 0;
  if (need === 0) return a.setStatus('Nothing needs work');
  ask({
    head: `Identify ${plural(need, 'track')} on MusicBrainz?`,
    body: ['Each one is searched for, and the best match is taken only if it is a confident one (a score of 95 or more). The rest are left alone for you to look at.',
      'This takes about a second per track, and u undoes it all in one step.'],
    yes: 'identify',
    onYes: startIdentifyAll,
  });
}

async function startIdentifyAll() {
  let view;
  try {
    view = await post('/api/identify-all');
  } catch (err) {
    return a.fail(err);
  }
  let job = view;
  const overlay = {
    title: 'Identify all',
    render(body) {
      const p = job.data || { total: 0, done: 0, identified: 0, left: 0, failed: 0, failures: [] };
      const head = job.state === 'running' ? 'Identifying tracks on MusicBrainz…' : job.state === 'stopped' ? 'Stopped' : job.state === 'failed' ? job.error : 'Done';
      body.append(h('div', { class: 'ask-head' }, head), progress(p.done, p.total));
      body.append(h('div', {}, `Identified ${p.identified} · Left for you ${p.left} · Failed ${p.failed}`));
      (p.failures || []).slice(0, 3).forEach((f) => body.append(h('div', { class: 'warn' }, f)));
      if ((p.failures || []).length > 3) body.append(h('div', { class: 'dim' }, '…and more'));
      if (job.state !== 'running' && p.left > 0) body.append(h('div', { class: 'ask-body' }, 'The rest need a closer look: i searches for one track, o uses its old tags.'));
    },
    onKey(e) {
      if (e.key !== 'Escape' && e.key !== 'Enter') return false;
      if (job.state === 'running') stopJob(job.id); else closeOverlay();
      return true;
    },
    hints: () => (job.state === 'running' ? [['esc', 'stop']] : [['enter', 'close']]),
  };
  openOverlay(overlay);
  const end = await follow(view, (v) => { job = v; if (S.overlay === overlay) overlay.redraw(); });
  job = end;
  await a.loadLibrary(); // takes the new history, and the tracks as they are now
  if (S.overlay === overlay) overlay.redraw();
}


// ---- adopt the old tags

export async function adopt() {
  const r = tree.cursorRow();
  let ids = [];
  if (S.marks.size) ids = tree.markedIds();
  else if (r && (r.kind === 'track' || r.key === 'needs')) ids = tree.idsOf(r);
  if (ids.length === 0) return a.setStatus('Select a track that needs work, or the Needs work group', 'bad');
  await attempt(() => post('/api/adopt', { ids }));
}

// ---- add from a link

const ADD_FIELDS = [['title', 'Title'], ['artist', 'Artist'], ['album', 'Album'], ['track', 'Track'], ['of', 'of'], ['released', 'Released'], ['genre', 'Genre'], ['cutStart', 'Cut start'], ['cutEnd', 'Cut end']];
const DEFAULTED = ['title', 'artist', 'album', 'track', 'of', 'released'];

export function add() {
  if (!S.info || !S.info.canAdd) return a.setStatus('Adding from YouTube is not available: yt-dlp is not set up', 'bad');
  let state = 'asking'; // asking, fetching, form, saving, duplicate, failed
  let link = '';
  let error = '';
  let job = null;
  let started = 0;
  let timer = 0;
  let draft = null; // what the download came to
  let useMatch = true;
  const form = Object.fromEntries(ADD_FIELDS.map(([k]) => [k, '']));
  const touched = new Set();
  let existing = null;
  let focus = 0;
  let pathNote = '';
  let pathTimer = 0;
  let linkInput = null;
  let inputs = {};
  let finished = false;

  // what the download says goes into the fields the person has not typed in
  const fill = (values) => { for (const k of DEFAULTED) if (!touched.has(k)) form[k] = values[k]; };

  async function fetchLink() {
    link = (linkInput ? linkInput.value : link).trim();
    if (!link) { error = 'Paste a link first'; overlay.redraw(); return; }
    state = 'fetching';
    error = '';
    started = Date.now();
    overlay.redraw();
    timer = setInterval(() => { if (state === 'fetching') overlay.redraw(); }, 1000);
    try {
      job = await post('/api/add/fetch', { url: link });
    } catch (err) {
      clearInterval(timer);
      error = err.message;
      state = 'failed';
      overlay.redraw();
      return;
    }
    const end = await follow(job, (v) => { job = v; });
    clearInterval(timer);
    if (S.overlay !== overlay || state !== 'fetching') return; // closed or cancelled meanwhile
    if (end.state === 'stopped') return;
    if (end.state === 'failed') { error = end.error; state = 'failed'; return overlay.redraw(); }
    const out = end.data;
    if (out.duplicate) { existing = out.duplicate; state = 'duplicate'; return overlay.redraw(); }
    draft = out.draft;
    useMatch = !!draft.match;
    touched.clear();
    fill(useMatch ? draft.matched : draft.downloaded);
    Object.assign(form, { genre: '', cutStart: '', cutEnd: '' });
    focus = 0;
    state = 'form';
    overlay.redraw();
    previewPath();
  }

  function previewPath() {
    clearTimeout(pathTimer);
    pathTimer = setTimeout(async () => {
      try {
        const res = await post('/api/add/path', { form });
        pathNote = res.path;
      } catch { pathNote = ''; }
      const el = document.getElementById('add-path');
      if (el) el.textContent = pathNote ? `→ ${pathNote}` : '';
    }, 120);
  }

  async function save() {
    state = 'saving';
    error = '';
    overlay.redraw();
    try {
      await change(() => post('/api/add/finish', { draft: draft.draft, form }));
      finished = true;
      closeOverlay();
    } catch (err) {
      error = err.message;
      state = 'form';
      overlay.redraw();
    }
  }

  function leave() {
    clearInterval(timer);
    clearTimeout(pathTimer);
    if (job && state === 'fetching') stopJob(job.id);
    closeOverlay();
  }

  const overlay = {
    title: 'Add from YouTube',
    render(body) {
      if (state === 'asking') {
        linkInput = h('input', { class: 'edit', type: 'text', autocomplete: 'off', spellcheck: 'false', placeholder: 'https://www.youtube.com/watch?v=…', 'aria-label': 'Link' });
        linkInput.value = link;
        const files = h('input', { type: 'file', accept: '.mp3,audio/mpeg', multiple: true, class: 'sr-only', 'aria-label': 'MP3 files', onchange: (ev) => { const f = [...ev.target.files]; if (f.length) { closeOverlay(); uploadFiles(f); } } });
        body.append(h('div', { class: 'ask-body' }, 'Paste a link to a video or a track:'), h('div', { class: 'form' }, linkInput));
        if (error) body.append(h('div', { class: 'bad' }, error));
        body.append(h('div', { class: 'buttons' }, h('button', { type: 'button', class: 'btn', onclick: () => files.click() }, 'Choose MP3 files instead…'), files),
          h('div', { class: 'ask-body' }, 'MP3 files can also be dropped on the page.'));
      } else if (state === 'fetching') {
        body.append(h('div', { class: 'busy' }, `Downloading… ${Math.floor((Date.now() - started) / 1000)}s`), h('div', { class: 'ask-body' }, 'This also looks the track up on MusicBrainz. esc cancels.'));
      } else if (state === 'saving') {
        body.append(h('div', { class: 'busy' }, 'Adding to the library…'));
      } else if (state === 'duplicate') {
        body.append(h('div', { class: 'warn' }, 'This audio is already in your library:'), h('div', { class: 'ask-head' }, name(existing.title), h('span', { class: 'dim' }, ' · ', name(existing.artist))));
      } else if (state === 'failed') {
        body.append(h('div', { class: 'bad' }, error));
      } else {
        const m = draft.match;
        body.append(!m ? h('div', { class: 'dim' }, "No match on MusicBrainz: these are YouTube's words.")
          : useMatch ? h('div', { class: 'accent' }, name(`Matched on MusicBrainz: ${m.artist} - ${m.title} · ${m.album}`))
            : h('div', { class: 'dim' }, "Ignoring the match: these are YouTube's words."));
        inputs = {};
        const grid = h('div', { class: 'form' });
        for (const [k, label] of ADD_FIELDS) {
          if (k === 'of') continue;
          const input = h('input', {
            class: 'edit', type: 'text', dir: 'auto', autocomplete: 'off', spellcheck: 'false', 'aria-label': label,
            oninput: (ev) => { form[k] = ev.target.value; touched.add(k); previewPath(); },
            onfocus: () => { focus = ADD_FIELDS.findIndex(([x]) => x === k); },
          });
          input.value = form[k];
          inputs[k] = input;
          if (k === 'track') {
            const of = h('input', { class: 'edit short', type: 'text', inputmode: 'numeric', autocomplete: 'off', 'aria-label': 'Tracks on album',
              oninput: (ev) => { form.of = ev.target.value; touched.add('of'); previewPath(); }, onfocus: () => { focus = ADD_FIELDS.findIndex(([x]) => x === 'of'); } });
            of.value = form.of;
            inputs.of = of;
            grid.append(h('label', { class: 'label' }, label, h('span', { class: 'pair' }, input, h('span', { class: 'dim' }, ' of '), of)));
          } else {
            grid.append(h('label', { class: 'label' }, label, input));
          }
        }
        body.append(grid, h('div', { id: 'add-path', class: 'accent' }, pathNote ? `→ ${pathNote}` : ''));
        if (error) body.append(h('div', { class: 'bad' }, error));
      }
    },
    afterDraw() {
      if (state === 'asking' && linkInput) linkInput.focus();
      if (state === 'form') (inputs[ADD_FIELDS[focus][0]] || inputs.title).focus();
    },
    onKey(e) {
      if (state === 'asking') {
        if (e.key === 'Escape') leave(); else if (e.key === 'Enter') fetchLink(); else { error = ''; return false; }
      } else if (state === 'fetching' || state === 'saving') {
        if (e.key === 'Escape') leave(); else return false;
      } else if (state === 'duplicate') {
        if (e.key === 'Escape') leave();
        else if (e.key === 'Enter') { closeOverlay(); a.loadLibrary({ reveal: existing.id }); } else return false;
      } else if (state === 'failed') {
        if (e.key === 'Escape') leave(); else if (e.key === 'Enter') { state = 'asking'; fetchLink(); } else return false;
      } else if (e.key === 'Escape') leave();
      else if (e.key === 'Enter') save();
      else if (e.key === 'F2' && draft.match) {
        useMatch = !useMatch;
        fill(useMatch ? draft.matched : draft.downloaded);
        overlay.redraw();
        previewPath();
      } else if (e.key === 'Tab' || e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        const step = e.key === 'ArrowUp' || (e.key === 'Tab' && e.shiftKey) ? -1 : 1;
        focus = (focus + step + ADD_FIELDS.length) % ADD_FIELDS.length;
        (inputs[ADD_FIELDS[focus][0]]).focus();
      } else { error = ''; return false; }
      return true;
    },
    onClose() {
      clearInterval(timer);
      clearTimeout(pathTimer);
      if (draft && !finished) del2(draft.draft);
    },
    hints() {
      if (state === 'asking') return [['enter', 'fetch'], ['esc', 'close']];
      if (state === 'form') return [['enter', 'add'], ['tab', 'next field'], ...(draft.match ? [['F2', useMatch ? 'ignore the match' : 'use the match']] : []), ['esc', 'close']];
      if (state === 'duplicate') return [['enter', 'show it'], ['esc', 'close']];
      if (state === 'failed') return [['enter', 'tries again'], ['esc', 'close']];
      return [['esc', 'close']];
    },
  };
  openOverlay(overlay);
}

// del2 forgets a download that is not going to be added.
function del2(draft) {
  fetch(`/api/add/${enc(draft)}`, { method: 'DELETE', headers: { 'X-MM': '1' } }).catch(() => {});
}

// ---- adding files

// uploadFiles adds the MP3 files chosen or dropped.
export async function uploadFiles(files) {
  const mp3 = files.filter((f) => /\.mp3$/i.test(f.name));
  if (mp3.length === 0) return a.setStatus('Only MP3 files can be added', 'bad');
  const body = new FormData();
  for (const f of mp3) body.append('files', f, f.name);
  a.setStatus(`Adding ${plural(mp3.length, 'file')}…`);
  await attempt(() => post('/api/ingest', body));
}

// dropFiles is what a drop on the page does: MP3 files are added, a picture becomes a cover.
export function dropFiles(files) {
  const list = [...files];
  if (list.some((f) => /\.mp3$/i.test(f.name))) return uploadFiles(list);
  const picture = pictureIn(list);
  if (picture) return dropPicture(picture);
  a.setStatus('Drop MP3 files to add them, or a JPEG or PNG picture to make it a cover', 'bad');
}

// ---- the library check

export function doctor() {
  let state = 'checking';
  let result = null;
  let error = '';
  let cursor = 0;
  const ctl = new AbortController();
  get('/api/doctor', { signal: ctl.signal }).then((v) => { result = v; state = 'done'; overlay.redraw(); },
    (err) => { if (err.name === 'AbortError') return; error = err.message; state = 'failed'; overlay.redraw(); });
  const go = () => {
    if (!result || result.problems.length === 0) return;
    const id = result.problems[cursor].trackId;
    closeOverlay();
    a.loadLibrary({ reveal: id });
  };
  const overlay = {
    title: 'Library check',
    render(body) {
      if (state === 'checking') return body.append(h('div', { class: 'busy' }, 'Checking that every stored file is there and unharmed…'), h('div', { class: 'ask-body' }, 'esc cancels'));
      if (state === 'failed') return body.append(h('div', { class: 'bad' }, error));
      if (result.problems.length === 0) return body.append(h('div', { class: 'good' }, 'No problems found.'));
      body.append(h('div', { class: 'ask-head' }, `${plural(result.errors, 'error')}, ${plural(result.warnings, 'warning')}`));
      const list = h('div', { class: 'problems', role: 'listbox' });
      result.problems.forEach((p, i) => {
        list.append(h('div', { class: `problem${i === cursor ? ' on' : ''}`, role: 'option', 'aria-selected': String(i === cursor), onclick: () => { cursor = i; overlay.redraw(); }, ondblclick: go },
          h('span', { class: p.severity === 'error' ? 'bad' : 'warn' }, p.severity === 'error' ? '✗' : '!'), h('b', {}, name(p.title)), h('span', { class: 'dim' }, ' · ', name(p.message))));
      });
      body.append(list);
      const on = list.querySelector('.on');
      if (on) queueMicrotask(() => on.scrollIntoView({ block: 'nearest' }));
    },
    onKey(e) {
      if (e.key === 'Escape' || e.key === 'q') closeOverlay();
      else if (state !== 'done' || result.problems.length === 0) return false;
      else if (e.key === 'ArrowUp' || e.key === 'k') { cursor = moveIn(result.problems.length, cursor, -1); overlay.redraw(); }
      else if (e.key === 'ArrowDown' || e.key === 'j') { cursor = moveIn(result.problems.length, cursor, 1); overlay.redraw(); }
      else if (e.key === 'Enter') go();
      else return false;
      return true;
    },
    onClose() { ctl.abort(); },
    hints: () => (state === 'done' && result.problems.length ? [['↑↓', 'choose'], ['enter', 'go to the track'], ['esc', 'close']] : [['esc', 'close']]),
  };
  openOverlay(overlay);
}

