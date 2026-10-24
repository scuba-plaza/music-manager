import { S } from './state.js';
import { get, post } from './api.js';
import { h, name, clear, plural } from './dom.js';
import { openOverlay, closeOverlay } from './overlay.js';

// The small dialogs that editing opens: a question, the form for several
// tracks, the list of what to copy, the choice of a playlist. Each calls back
// with what was chosen; the callback says whether it worked, and the dialog
// stays open with the reason if it did not.

const moveIn = (list, i, d) => Math.min(Math.max(i + d, 0), list.length - 1);

// ask puts a question that only y or enter go ahead with.
export function ask({ head, body = [], yes = 'ok', onYes }) {
  openOverlay({
    title: 'Are you sure?',
    render(el) {
      el.append(h('div', { class: 'ask-head' }, name(head)));
      for (const line of body) el.append(h('div', { class: line.startsWith('•') ? 'ask-item' : 'ask-body' }, name(line)));
      el.append(h('div', { class: 'buttons' },
        h('button', { type: 'button', class: 'btn primary', onclick: () => go() }, yes),
        h('button', { type: 'button', class: 'btn', onclick: closeOverlay }, 'Cancel')));
    },
    onKey(e) {
      if (e.key === 'y' || e.key === 'Enter') go();
      else if (e.key === 'Escape' || e.key === 'n' || e.key === 'q') closeOverlay();
      else return false;
      return true;
    },
    hints: () => [['y/enter', yes], ['esc', 'close']],
  });
  function go() {
    closeOverlay();
    onYes();
  }
}

// ---- several tracks at once

const BULK = [
  ['artist', 'Artist'], ['artistLatin', 'Artist (Latin)'], ['album', 'Album'], ['albumLatin', 'Album (Latin)'],
  ['of', 'Tracks on album'], ['released', 'Released'], ['genre', 'Genre'], ['genreLatin', 'Genre (Latin)'],
  ['cutStart', 'Cut start'], ['cutEnd', 'Cut end'],
];

// bulkForm edits the tracks with the given IDs together: blank keeps what they
// have, and - clears. apply(values) resolves to an error text, or '' when done.
export function bulkForm({ head, ids, preview, apply }) {
  const values = Object.fromEntries(BULK.map(([k]) => [k, '']));
  let focus = 0;
  let summary = { text: 'Type what the tracks should have.', bad: false };
  let timer = 0;
  let seq = 0;
  const inputs = BULK.map(([k, label]) => h('input', {
    class: 'edit', type: 'text', dir: 'auto', autocomplete: 'off', spellcheck: 'false', placeholder: '(keep)', 'aria-label': label,
    oninput: (ev) => { values[k] = ev.target.value; scheduleSummary(); },
    onfocus: () => { focus = inputs.indexOf(document.activeElement); },
  }));
  const line = h('div', { class: 'bulk-summary' });
  const showSummary = () => {
    line.textContent = summary.text;
    line.className = `bulk-summary${summary.bad ? ' bad' : ''}`;
  };
  function scheduleSummary() {
    clearTimeout(timer);
    timer = setTimeout(async () => {
      const mine = ++seq;
      try {
        const p = await preview(ids, values);
        if (mine !== seq) return;
        summary = p.error ? { text: p.error, bad: true } : { text: p.summary || 'Type what the tracks should have.', bad: false };
      } catch (err) {
        if (mine === seq) summary = { text: err.message, bad: true };
      }
      showSummary();
    }, 150);
  }
  const focusInput = (i) => {
    focus = (i + inputs.length) % inputs.length;
    inputs[focus].focus();
  };
  const submit = async () => {
    const err = await apply(ids, values);
    if (err) {
      summary = { text: err, bad: true };
      showSummary();
    }
  };
  openOverlay({
    title: head,
    render(body) {
      body.append(h('div', { class: 'ask-body' }, `Applies to ${plural(ids.length, 'track')}. Blank keeps what they have; - clears.`));
      const grid = h('div', { class: 'form' });
      BULK.forEach(([, label], i) => grid.append(h('label', { class: 'label' }, label, inputs[i])));
      body.append(grid, line);
      showSummary();
    },
    afterDraw: () => inputs[focus].focus(),
    onKey(e) {
      if (e.key === 'Escape') closeOverlay();
      else if (e.key === 'Enter') submit();
      else if (e.key === 'Tab') focusInput(focus + (e.shiftKey ? -1 : 1));
      else if (e.key === 'ArrowDown') focusInput(focus + 1);
      else if (e.key === 'ArrowUp') focusInput(focus - 1);
      else return false;
      return true;
    },
    hints: () => [['enter', 'apply'], ['tab', 'next field'], ['esc', 'close']],
  });
}

// ---- what to copy from a track

export async function copyList({ id, title, onCopy }) {
  const choices = await get(`/api/tracks/${encodeURIComponent(id)}/copyable`);
  const ticked = choices.map((c) => c.has);
  let cursor = Math.max(0, choices.findIndex((c) => c.has));
  let error = '';
  const submit = async () => {
    const attrs = choices.filter((c, i) => ticked[i]).map((c) => c.key);
    if (attrs.length === 0) {
      error = 'Tick what to copy';
      overlay.redraw();
      return;
    }
    const err = await onCopy(attrs);
    if (err) {
      error = err;
      overlay.redraw();
    }
  };
  const toggle = (i) => {
    if (!choices[i].has) return;
    ticked[i] = !ticked[i];
    error = '';
    overlay.redraw();
  };
  const overlay = {
    title: `Copy from ${title}`,
    render(body) {
      const list = h('div', { class: 'menu', role: 'listbox' });
      choices.forEach((c, i) => {
        list.append(h('div', {
          class: `item${i === cursor ? ' on' : ''}${c.has ? '' : ' off'}`, role: 'option', 'aria-selected': String(i === cursor),
          onclick: () => { cursor = i; toggle(i); },
        }, h('span', { class: 'n' }, c.has ? (ticked[i] ? '[x]' : '[ ]') : '   '), h('span', { class: 't' }, c.label), h('span', { class: 'd' }, c.has ? name(c.text) : '—')));
      });
      body.append(list);
      if (error) body.append(h('div', { class: 'bad' }, error));
    },
    onKey(e) {
      if (e.key === 'Escape' || e.key === 'q') closeOverlay();
      else if (e.key === 'ArrowUp' || e.key === 'k') { cursor = moveIn(choices, cursor, -1); overlay.redraw(); }
      else if (e.key === 'ArrowDown' || e.key === 'j') { cursor = moveIn(choices, cursor, 1); overlay.redraw(); }
      else if (e.key === ' ') toggle(cursor);
      else if (e.key === 'Enter') submit();
      else return false;
      return true;
    },
    hints: () => [['space', 'ticks'], ['enter', 'copies'], ['esc', 'close']],
  };
  openOverlay(overlay);
}

// ---- which playlist

// playlistPicker chooses the playlist to add the tracks to, or asks for the
// name of a new one. choose(name, isNew) resolves to an error text or ''.
export function playlistPicker({ count, choose }) {
  const lists = S.lib.playlists;
  let cursor = 0; // lists.length is the entry that makes a new one
  let naming = lists.length === 0;
  let typed = '';
  let error = '';
  let input = null;
  const submit = async () => {
    const pickName = naming ? typed.trim() : lists[cursor].name;
    if (!pickName) { error = 'Type a name'; overlay.redraw(); return; }
    const err = await choose(pickName, naming);
    if (err) {
      error = err;
      overlay.redraw();
    }
  };
  const overlay = {
    title: 'Add to a playlist',
    render(body) {
      body.append(h('div', { class: 'ask-body' }, `Adding ${plural(count, 'track')}`));
      if (naming) {
        input = h('input', {
          class: 'edit', type: 'text', dir: 'auto', maxlength: '80', placeholder: 'name of the playlist', 'aria-label': 'Name of the playlist',
          oninput: (ev) => { typed = ev.target.value; error = ''; },
        });
        input.value = typed;
        body.append(h('div', { class: 'form' }, input));
      } else {
        const list = h('div', { class: 'menu', role: 'listbox' });
        [...lists.map((p) => [p.name, `${p.count}`]), ['New playlist…', '']].forEach(([label, right], i) => {
          list.append(h('div', {
            class: `item${i === cursor ? ' on' : ''}`, role: 'option', 'aria-selected': String(i === cursor),
            onclick: () => { cursor = i; pick(); },
          }, h('span', { class: 't' }, name(label)), h('span', { class: 'd' }, right)));
        });
        body.append(list);
      }
      if (error) body.append(h('div', { class: 'bad' }, error));
    },
    afterDraw: () => { if (input && naming) input.focus(); },
    onKey(e) {
      if (e.key === 'Escape') {
        if (naming && lists.length > 0) { naming = false; error = ''; overlay.redraw(); } else closeOverlay();
      } else if (e.key === 'Enter') pick();
      else if (!naming && (e.key === 'ArrowUp' || e.key === 'k')) { cursor = moveIn(new Array(lists.length + 1), cursor, -1); overlay.redraw(); }
      else if (!naming && (e.key === 'ArrowDown' || e.key === 'j')) { cursor = moveIn(new Array(lists.length + 1), cursor, 1); overlay.redraw(); }
      else if (!naming && e.key === 'n') { naming = true; overlay.redraw(); }
      else if (!naming && e.key === 'q') closeOverlay();
      else return false;
      return true;
    },
    hints: () => (naming ? [['enter', 'make it'], ['esc', lists.length ? 'back' : 'close']] : [['enter', 'add'], ['n', 'new'], ['esc', 'close']]),
  };
  function pick() {
    if (!naming && cursor === lists.length) { naming = true; overlay.redraw(); return; }
    submit();
  }
  openOverlay(overlay);
}

export { closeOverlay, clear };
