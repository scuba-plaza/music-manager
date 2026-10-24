import { S, saveTab } from './state.js';
import { h, name, clear } from './dom.js';

// The right pane: the fields of a track, or what is said about an artist, an
// album or a playlist. What to show comes from the server ready-made; this
// file lays it out. A field being typed in is an input in its place.

export const hooks = {
  gotoKey() {}, tabChosen() {}, fieldChosen() {},
  // while a field is typed in: the keys that save and cancel, and the text changing
  save() {}, cancel() {}, typed() {},
};

const el = () => document.getElementById('inspector');

// tabIndex is the tab shown for a track view, kept within what the track has.
export function tabIndex(view) {
  return Math.min(S.tab, view.tabs.length - 1);
}

// fieldsOfTab lists the fields of the tab shown.
export function fieldsOfTab(view) {
  return view.tabs[tabIndex(view)].fields;
}

// fieldList lists the fields of what the cursor is on: those of the tab of a
// track, or of an artist, an album or a playlist.
export function fieldList() {
  const d = S.detail;
  if (!d) return [];
  if (d.kind === 'track') return fieldsOfTab(d.data);
  if (d.kind === 'peek') return d.data.fields || [];
  return [];
}

// currentField is the field the cursor is on in the pane, if the keyboard is there.
export function currentField() {
  if (S.focus !== 'inspector') return null;
  return fieldList()[S.field] || null;
}

export function renderInspector() {
  const root = clear(el());
  const d = S.detail;
  if (!d) {
    if (S.lib && S.lib.tracks > 0) root.append(h('div', { class: 'empty' }, 'Nothing selected'));
    setTitle('Details');
    return;
  }
  if (d.kind === 'error') {
    root.append(h('div', { class: 'empty bad' }, d.message));
    return;
  }
  if (d.kind === 'track') renderTrack(root, d.data);
  else renderPeek(root, d.data);
  focusEditor(root);
}

// focusEditor puts the caret in the field being typed in, at the end of the text.
function focusEditor(root) {
  if (!S.edit) return;
  const inputs = root.querySelectorAll('input.edit');
  const input = inputs[S.edit.second ? 1 : 0] || inputs[0];
  if (!input) return;
  input.focus({ preventScroll: true });
  input.setSelectionRange(input.value.length, input.value.length);
}

function setTitle(text) {
  document.getElementById('right-title').textContent = text;
}

function tabStrip(view) {
  const strip = h('div', { class: 'tabs', role: 'tablist' });
  view.tabs.forEach((tab, i) => {
    strip.append(h('button', {
      type: 'button', role: 'tab', class: `tab${i === tabIndex(view) ? ' on' : ''}`, 'aria-selected': String(i === tabIndex(view)),
      onclick: () => { S.tab = i; saveTab(); S.field = 0; hooks.tabChosen(); },
    }, `${i + 1} ${tab.name}`));
  });
  return strip;
}

// editing says whether field i of this pane is the one being typed in.
const editing = (scope, i) => S.edit && S.edit.scope === scope && S.edit.index === i;

// fieldRows adds the label and the value of one field to the grid.
function fieldRow(grid, f, i, scope) {
  const cur = S.focus === 'inspector' && i === S.field;
  grid.append(h('div', { class: `label${cur ? ' cur' : ''}`, onclick: () => hooks.fieldChosen(i) }, f.label));
  if (editing(scope, i)) {
    grid.append(h('div', { class: 'value cur editing', 'data-field': f.key }, editor(S.edit)));
    return;
  }
  const text = f.text !== '' ? f.text : f.empty || '';
  grid.append(h('div', {
    class: `value${f.dim || f.text === '' || f.editable === false ? ' dim' : ''}${cur ? ' cur' : ''}`, 'data-field': f.key,
    onclick: () => hooks.fieldChosen(i),
  }, f.text !== '' ? name(text) : text));
}

// hintLine is what may be typed in the field, under it while it is the current one.
function hintLine(grid, f, i, scope) {
  const cur = S.focus === 'inspector' && i === S.field;
  if (!cur) return;
  if (editing(scope, i)) {
    grid.append(h('div', { class: 'span hintline' }, f.hint || ''));
    grid.append(h('div', { id: 'edit-note', class: 'span note-live' }, S.edit.note || ''));
    grid.append(h('div', { id: 'edit-error', class: 'span bad' }, S.edit.error || ''));
  } else if (f.hint) {
    grid.append(h('div', { class: 'span hintline' }, f.hint));
  }
}

// editor is the input (or the two, for the number of a track) a field is typed in.
function editor(e) {
  const keys = (ev) => {
    if (ev.key === 'Enter') {
      ev.preventDefault();
      hooks.save();
    } else if (ev.key === 'Escape') {
      ev.preventDefault();
      hooks.cancel();
    } else if (ev.key === 'Tab' && e.hasSecond) {
      ev.preventDefault();
      e.second = !e.second;
      const inputs = el().querySelectorAll('input.edit');
      inputs[e.second ? 1 : 0].focus();
    }
  };
  const first = h('input', {
    class: 'edit', type: 'text', dir: 'auto', autocomplete: 'off', spellcheck: 'false', 'aria-label': e.label,
    placeholder: e.mixed ? '(mixed)' : null,
    oninput: (ev) => hooks.typed('text', ev.target.value),
    onkeydown: keys,
  });
  first.value = e.text;
  if (!e.hasSecond) return first;
  const second = h('input', {
    class: 'edit short', type: 'text', inputmode: 'numeric', autocomplete: 'off', 'aria-label': 'Tracks on album',
    oninput: (ev) => hooks.typed('secondText', ev.target.value),
    onkeydown: keys,
  });
  second.value = e.secondText;
  return h('span', { class: 'pair' }, first, h('span', { class: 'dim' }, ' of '), second);
}

function renderTrack(root, v) {
  setTitle('Details');
  const tab = v.tabs[tabIndex(v)];
  root.append(tabStrip(v), h('div', { class: 'title' }, name(v.title)), h('hr'));
  for (const n of tab.notes || []) root.append(h('div', { class: `note${tab.warn ? ' warn' : ''}` }, n));

  const grid = h('div', { class: 'fields' });
  tab.fields.forEach((f, i) => {
    fieldRow(grid, f, i, 'track');
    if (f.key === 'cutEnd' && tab.bar) grid.append(h('div', { class: 'label' }, 'Kept'), keptBar(tab.bar));
    if (f.key === 'fadeOut' && tab.bar && tab.bar.level) grid.append(h('div', { class: 'label' }, 'Level'), levelBar(tab.bar));
    hintLine(grid, f, i, 'track');
  });
  if (tab.wolfgang) renderWolfgang(grid, tab.wolfgang);
  root.append(grid);

  if (tabIndex(v) === 0 && v.coverSha) root.append(coverBox(v.id, v.coverSha));
  root.append(h('hr'), h('div', { class: 'pathline' }, name(v.path)), h('div', { class: 'foot' }, v.dates), h('div', { class: 'foot' }, name(v.about)));
}

function coverBox(trackId, sha) {
  return h('div', { class: 'cover-box' }, h('img', {
    src: `/api/tracks/${encodeURIComponent(trackId)}/cover?v=${sha.slice(0, 16)}`, alt: 'Cover', loading: 'lazy',
  }));
}

function keptBar(b) {
  const total = Math.max(b.total, 1);
  const pct = (n) => `${(100 * n) / total}%`;
  const keptLen = Math.max(total - b.start - b.end, 0);
  return h('div', {},
    h('div', { class: 'bar', role: 'img', 'aria-label': b.kept },
      h('i', { class: 'cut', style: `flex:0 0 ${pct(b.start)}` }),
      h('i', { class: 'kept', style: `flex:0 0 ${pct(keptLen)}` }),
      h('i', { class: 'cut', style: `flex:0 0 ${pct(b.end)}` })),
    h('div', { class: 'hintline' }, b.kept));
}

function levelBar(b) {
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  svg.setAttribute('viewBox', '0 0 100 10');
  svg.setAttribute('preserveAspectRatio', 'none');
  svg.setAttribute('class', 'level');
  svg.setAttribute('role', 'img');
  svg.setAttribute('aria-label', 'Volume across the track, with the fades');
  let d = '';
  b.level.forEach((v, i) => {
    if (v < 0) return;
    const hh = Math.max(v * 10, 0.3);
    d += `M${i} ${10 - hh}h1v${hh}h-1z`;
  });
  const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
  path.setAttribute('d', d);
  svg.append(path);
  return svg;
}

function renderWolfgang(grid, w) {
  if (w.notice) grid.append(h('div', { class: 'span warn' }, w.notice));
  for (const r of w.rows) {
    grid.append(h('div', { class: 'label' }, r.label), h('div', { class: 'value' }, name(r.value), r.tail ? h('span', { class: 'dim' }, ` ${r.tail}`) : null));
    if (r.hint) grid.append(h('div', { class: 'span hintline' }, r.hint));
  }
  for (const m of w.warnings || []) grid.append(h('div', { class: 'span warn' }, `! ${m}`));
  grid.append(h('div', { class: 'span dim' }, w.footnote));
}

function renderPeek(root, p) {
  setTitle(p.title);
  root.append(h('div', { class: 'title' }, name(p.label)), h('hr'));
  for (const line of p.info) root.append(h('div', { class: 'note' }, name(line)));
  if (p.path) root.append(h('div', { class: 'pathline' }, name(p.path)));
  if (p.fields && p.fields.length) {
    const grid = h('div', { class: 'fields' });
    p.fields.forEach((f, i) => {
      fieldRow(grid, f, i, 'group');
      hintLine(grid, f, i, 'group');
    });
    root.append(grid);
    if (p.note) root.append(h('div', { class: 'note dim' }, p.note));
    if (p.coverTrack) root.append(coverBox(p.coverTrack, p.coverSha));
    root.append(h('hr'));
  }
  if (p.list.length) {
    const grid = h('div', { class: 'listrows' });
    for (const r of p.list) {
      const row = h('div', { class: 'lr', onclick: () => hooks.gotoKey(r.key), role: 'button', tabindex: '-1' });
      row.append(h('div', { class: `l${r.dim ? ' dim' : ''}` }, name(r.left)));
      const cols = r.cols || [];
      row.append(h('div', { class: 'c' }, cols.length > 1 ? name(cols[0]) : ''), h('div', { class: 'c' }, cols[cols.length - 1] || ''));
      grid.append(row);
    }
    root.append(grid);
  }
}
