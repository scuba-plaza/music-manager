import { S, saveTab } from './state.js';
import { h, name, clear } from './dom.js';

// The right pane: the fields of a track, or what is said about an artist, an
// album or a playlist. What to show comes from the server ready-made; this
// file lays it out.

export const hooks = { gotoKey() {}, tabChosen() {}, fieldChosen() {} };

const el = () => document.getElementById('inspector');

// tabIndex is the tab shown for a track view, kept within what the track has.
export function tabIndex(view) {
  return Math.min(S.tab, view.tabs.length - 1);
}

// fieldsOfTab lists the fields of the tab shown.
export function fieldsOfTab(view) {
  return view.tabs[tabIndex(view)].fields;
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

function renderTrack(root, v) {
  setTitle('Details');
  const tab = v.tabs[tabIndex(v)];
  root.append(tabStrip(v), h('div', { class: 'title' }, name(v.title)), h('hr'));
  for (const n of tab.notes || []) root.append(h('div', { class: `note${tab.warn ? ' warn' : ''}` }, n));

  const grid = h('div', { class: 'fields' });
  tab.fields.forEach((f, i) => {
    const cur = S.focus === 'inspector' && i === S.field;
    const label = h('div', { class: `label${cur ? ' cur' : ''}`, onclick: () => hooks.fieldChosen(i) }, f.label);
    const text = f.text !== '' ? f.text : f.empty || '';
    const value = h('div', {
      class: `value${f.dim || f.text === '' ? ' dim' : ''}${cur ? ' cur' : ''}`, 'data-field': f.key,
      onclick: () => hooks.fieldChosen(i),
    }, f.text !== '' ? name(text) : text);
    grid.append(label, value);
    if (f.key === 'cutEnd' && tab.bar) grid.append(h('div', { class: 'label' }, 'Kept'), keptBar(tab.bar));
    if (f.key === 'fadeOut' && tab.bar && tab.bar.level) grid.append(h('div', { class: 'label' }, 'Level'), levelBar(tab.bar));
    if (cur && f.hint) grid.append(h('div', { class: 'span hintline' }, f.hint));
  });
  if (tab.wolfgang) renderWolfgang(grid, tab.wolfgang);
  root.append(grid);

  if (tabIndex(v) === 0 && v.coverSha) {
    root.append(h('div', { class: 'cover-box' }, h('img', {
      src: `/api/tracks/${encodeURIComponent(v.id)}/cover?v=${v.coverSha.slice(0, 16)}`, alt: 'Cover', loading: 'lazy',
    })));
  }
  root.append(h('hr'), h('div', { class: 'pathline' }, name(v.path)), h('div', { class: 'foot' }, v.dates), h('div', { class: 'foot' }, name(v.about)));
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
