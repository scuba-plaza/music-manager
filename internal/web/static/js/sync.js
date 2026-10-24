import { S } from './state.js';
import { get, post, del } from './api.js';
import { h, name, plural, progress } from './dom.js';
import * as a from './actions.js';
import { openOverlay, closeOverlay } from './overlay.js';
import { follow, stopJob } from './jobs.js';

// The sync dialog writes the library to a folder or a drive: choose where,
// look at what would change, then apply it. Nothing is written until the
// person has seen the plan, and files are only deleted after they typed
// "delete".

const enc = encodeURIComponent;
const GLYPH = { create: '+', update: '~', remove: '−', stray: '!' };
const WARNINGS_SHOWN = 4;

export async function openSync() {
  if (!S.lib) return;
  let targets;
  try {
    targets = (await get('/api/sync/targets')).targets;
  } catch (err) {
    return a.fail(err);
  }
  targets.push({ label: 'Another folder…', path: '', kind: 'folder' });

  let state = 'choosing'; // choosing, typing, planning, reviewing, confirming, applying, finished, failed
  let cursor = 0;
  let wolfgang = !!(S.info && S.info.wolfgang); // a drive starts on the format that was asked for
  let folder = '';
  let error = '';
  let where = '';
  let job = null;
  let plan = null; // what the server says the plan is: ops, counts, warnings
  let summary = '';
  let confirm = '';
  let input = null;
  let list = null; // the scrollable list of operations
  let attempt = 0; // which plan the dialog is waiting for

  const isLibrary = () => targets[cursor].kind === 'library';
  const isFolder = () => targets[cursor].kind === 'folder';

  async function start() {
    const t = targets[cursor];
    const typed = t.kind === 'folder';
    state = 'planning';
    error = '';
    where = typed ? folder.trim() : t.label;
    job = null;
    overlay.redraw();
    try {
      job = await post('/api/sync/plan', { library: t.kind === 'library', folder: typed ? folder : t.kind === 'drive' ? t.path : '', wolfgang: wolfgang && t.kind !== 'library' });
    } catch (err) {
      error = err.message;
      state = typed ? 'typing' : 'failed';
      return overlay.redraw();
    }
    const mine = ++attempt;
    const end = await follow(job, (v) => { job = v; if (state === 'planning' && S.overlay === overlay) overlay.redraw(); });
    if (mine !== attempt || state !== 'planning' || S.overlay !== overlay) return; // closed or cancelled meanwhile
    if (end.state === 'stopped') return;
    if (end.state === 'failed') { error = end.error; state = 'failed'; return overlay.redraw(); }
    plan = end.data.plan;
    state = 'reviewing';
    overlay.redraw();
    if (t.kind === 'library' && plan.ops.length === 0) a.loadLibrary(); // the folder was found up to date: the header's count goes
  }

  async function strays() {
    error = '';
    try {
      plan = await post(`/api/sync/${enc(plan.plan)}/prune`, { prune: !plan.prune });
    } catch (err) {
      error = err.message;
    }
    overlay.redraw();
  }

  function apply() {
    if (plan.ops.length === 0) return closeOverlay();
    if (plan.counts.remove > 0) {
      state = 'confirming';
      confirm = '';
      error = '';
      return overlay.redraw();
    }
    run();
  }

  async function run() {
    state = 'applying';
    error = '';
    overlay.redraw();
    let started;
    try {
      started = await post(`/api/sync/${enc(plan.plan)}/apply`, { confirm: confirm.trim() });
    } catch (err) {
      error = err.message;
      state = plan.counts.remove > 0 ? 'confirming' : 'failed';
      return overlay.redraw();
    }
    job = started;
    const end = await follow(started, (v) => { job = v; if (state === 'applying' && S.overlay === overlay) overlay.redraw(); });
    if (end.state === 'done') {
      summary = end.data.summary;
      state = 'finished';
    } else {
      error = end.error || 'The sync was stopped.';
      state = 'failed';
    }
    await a.loadLibrary(); // the history, the count in the header, and the tracks as they are now
    if (S.overlay === overlay) overlay.redraw();
    else if (state === 'finished') a.setStatus(`Synced to ${where}: ${summary}`, 'good');
    else a.setStatus(error, 'bad');
  }

  const section = (text) => h('div', { class: 'ask-body' }, text);

  const overlay = {
    title: 'Sync',
    render(body) {
      if (state === 'choosing') {
        body.append(section('Where should the music be written?'));
        const menu = h('div', { class: 'menu wide', role: 'listbox' });
        targets.forEach((t, i) => {
          menu.append(h('div', {
            class: `item${i === cursor ? ' on' : ''}`, role: 'option', 'aria-selected': String(i === cursor),
            onclick: () => { cursor = i; overlay.redraw(); }, ondblclick: choose,
          }, h('span', { class: 't' }, name(t.label), t.path ? h('span', { class: 'dim' }, '  ', name(t.path)) : null)));
        });
        body.append(menu, formatLine());
      } else if (state === 'typing') {
        body.append(section('Write the music into which folder?'));
        input = h('input', {
          class: 'edit', type: 'text', dir: 'auto', autocomplete: 'off', spellcheck: 'false', placeholder: '~/Music/player', 'aria-label': 'Folder',
          oninput: (ev) => { folder = ev.target.value; error = ''; const e = body.querySelector('.bad'); if (e) e.remove(); },
        });
        input.value = folder;
        body.append(h('div', { class: 'form' }, h('label', { class: 'label' }, 'Folder', input)), formatLine('click'));
        if (error) body.append(h('div', { class: 'bad' }, error));
      } else if (state === 'planning') {
        const p = job && job.data && job.data.phase === 'plan' ? job.data : { done: 0, total: 0 };
        body.append(h('div', { class: 'busy' }, `Preparing ${where}…`));
        if (p.total > 0) body.append(progress(p.done, p.total));
        body.append(section('Checking every file against its record; esc cancels.'));
      } else if (state === 'reviewing' || state === 'confirming') {
        review(body);
      } else if (state === 'applying') {
        const p = job && job.data && job.data.phase === 'apply' ? job.data : { done: 0, total: plan.ops.length };
        body.append(h('div', { class: 'busy' }, `Writing ${plan.where}…`), progress(p.done, p.total));
      } else if (state === 'finished') {
        body.append(h('div', {}, h('span', { class: 'good' }, 'Done'), ' ', name(plan.where)), h('div', { class: 'ask-head' }, summary));
      } else {
        body.append(h('div', { class: 'bad' }, error));
      }
    },
    afterDraw() {
      if ((state === 'typing' || state === 'confirming') && input) input.focus();
    },
    onKey(e) {
      switch (state) {
        case 'choosing':
          if (e.key === 'Escape' || e.key === 'q') closeOverlay();
          else if (e.key === 'ArrowUp' || e.key === 'k') { cursor = Math.max(cursor - 1, 0); overlay.redraw(); }
          else if (e.key === 'ArrowDown' || e.key === 'j') { cursor = Math.min(cursor + 1, targets.length - 1); overlay.redraw(); }
          else if (e.key === 'w') toggleFormat();
          else if (e.key === 'Enter') choose();
          else return false;
          return true;
        case 'typing':
          if (e.key === 'Escape') { state = 'choosing'; error = ''; overlay.redraw(); }
          else if (e.key === 'Enter') {
            if (!folder.trim()) { error = 'Type the folder to write into'; overlay.redraw(); } else start();
          } else return false;
          return true;
        case 'planning':
          if (e.key !== 'Escape') return false;
          if (job) stopJob(job.id);
          closeOverlay();
          return true;
        case 'reviewing':
          if (e.key === 'Escape' || e.key === 'q') closeOverlay();
          else if (e.key === 'ArrowUp' || e.key === 'k') scroll(-1);
          else if (e.key === 'ArrowDown' || e.key === 'j') scroll(1);
          else if (e.key === 'PageUp') scroll(-10);
          else if (e.key === 'PageDown') scroll(10);
          else if (e.key === 'Home') scroll(-Infinity);
          else if (e.key === 'End') scroll(Infinity);
          else if (e.key === 'p' && plan.ops.length > 0) strays();
          else if (e.key === 'Enter') apply();
          else return false;
          return true;
        case 'confirming':
          if (e.key === 'Escape') back();
          else if (e.key === 'Enter') confirmed();
          else return false;
          return true;
        case 'finished':
        case 'failed':
          if (e.key !== 'Escape' && e.key !== 'Enter') return false;
          closeOverlay();
          return true;
        default: // applying: nothing to press until it is over
          return false;
      }
    },
    onClose() {
      if (state === 'planning' && job) stopJob(job.id);
      if ((state === 'reviewing' || state === 'confirming') && plan) del(`/api/sync/${enc(plan.plan)}`).catch(() => {});
    },
    hints() {
      switch (state) {
        case 'choosing': return [['↑↓', 'choose'], ['enter', 'choose'], ...(isLibrary() ? [] : [['w', 'format']]), ['esc', 'close']];
        case 'typing': return [['enter', 'plan'], ['esc', 'back']];
        case 'planning': return [['esc', 'cancel']];
        case 'reviewing': return plan.ops.length === 0 ? [['enter', 'close']] : [['↑↓', 'scroll'], ['enter', 'apply'], ['p', 'strays'], ['esc', 'close']];
        case 'confirming': return [['enter', 'confirm'], ['esc', 'back']];
        case 'applying': return [];
        default: return [['enter', 'close']];
      }
    },
  };

  function choose() {
    if (isFolder()) { state = 'typing'; error = ''; return overlay.redraw(); }
    start();
  }

  function toggleFormat() {
    if (isLibrary()) return; // the library folder has no other format
    wolfgang = !wolfgang;
    overlay.redraw();
  }

  // formatLine says what kind of place the music goes to.
  function formatLine(how = 'w') {
    const library = isLibrary();
    const line = h('div', { class: 'format' }, h('span', { class: 'dim' }, 'Format  '));
    if (library) line.append('Generic ', h('span', { class: 'dim' }, '· the library folder is always written the standard way'));
    else if (wolfgang) line.append(h('span', { class: 'accent' }, 'Wolfgang WMP01'), ' ', h('span', { class: 'dim' }, `· ${how} for the standard way`));
    else line.append('Generic ', h('span', { class: 'dim' }, `· ${how} for another format`));
    if (!library) { line.classList.add('click'); line.addEventListener('click', toggleFormat); }
    return line;
  }

  function scroll(lines) {
    if (!list) return;
    const row = list.firstElementChild ? list.firstElementChild.getBoundingClientRect().height : 20;
    if (lines === Infinity) list.scrollTop = list.scrollHeight;
    else if (lines === -Infinity) list.scrollTop = 0;
    else list.scrollTop += lines * row;
  }

  function review(body) {
    list = null;
    if (plan.ops.length === 0) {
      body.append(h('div', {}, h('b', {}, 'Nothing to do'), h('span', { class: 'dim' }, ' · ', name(plan.where), ' is up to date.')));
      return;
    }
    body.append(h('div', {}, h('b', {}, plan.text), h('span', { class: 'dim' }, '  →  '), name(plan.where)));
    list = h('div', { class: 'ops', tabindex: '-1', 'aria-label': 'What would change' });
    for (const op of plan.ops) {
      list.append(h('div', { class: `op ${op.kind}`, title: op.path }, h('span', { class: 'g' }, GLYPH[op.kind]), name(op.path)));
    }
    body.append(list);
    const ws = plan.warnings;
    if (ws.length > 0) {
      body.append(h('div', { class: 'warn warns' }, `${plural(ws.length, 'warning')} for the player`));
      ws.slice(0, WARNINGS_SHOWN).forEach((w) => body.append(h('div', { class: 'dim' }, name(w.track), '  ', w.message)));
      if (ws.length > WARNINGS_SHOWN) body.append(h('div', { class: 'dim' }, `… ${ws.length - WARNINGS_SHOWN} more`));
    }
    if (plan.counts.stray > 0) body.append(section('Stray files are left alone. p also deletes the strays.'));
    else if (plan.counts.remove > 0) body.append(h('div', { class: 'warn' }, 'The files marked − will be deleted. p keeps them.'));
    if (state === 'confirming') {
      input = h('input', {
        class: 'edit short', type: 'text', autocomplete: 'off', spellcheck: 'false', 'aria-label': 'Type delete to confirm',
        oninput: (ev) => { confirm = ev.target.value; },
      });
      input.value = confirm;
      body.append(h('div', { class: 'confirm' }, h('span', { class: 'warn' }, `Type delete to remove ${plural(plan.counts.remove, 'file')}: `), input));
      if (error) body.append(h('div', { class: 'bad' }, error));
    } else if (error) {
      body.append(h('div', { class: 'bad' }, error));
    }
    if (state === 'confirming') {
      body.append(h('div', { class: 'buttons' },
        h('button', { type: 'button', class: 'btn primary', onclick: confirmed }, 'Delete and write'),
        h('button', { type: 'button', class: 'btn', onclick: back }, 'Back')));
      return;
    }
    body.append(h('div', { class: 'buttons' },
      h('button', { type: 'button', class: 'btn primary', onclick: apply }, 'Apply'),
      plan.counts.stray > 0 || plan.prune ? h('button', { type: 'button', class: 'btn', onclick: strays }, plan.prune ? 'Keep the strays' : 'Delete the strays') : null,
      h('button', { type: 'button', class: 'btn', onclick: closeOverlay }, 'Cancel')));
  }

  // confirmed goes ahead if the word was typed, and otherwise starts the word again.
  function confirmed() {
    if (confirm.trim() === 'delete') return run();
    confirm = '';
    if (input) { input.value = ''; input.focus(); }
  }

  function back() {
    state = 'reviewing';
    error = '';
    overlay.redraw();
  }

  openOverlay(overlay);
}
