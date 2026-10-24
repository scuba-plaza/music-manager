import { h } from './dom.js';
import { openOverlay, closeOverlay } from './overlay.js';

// Every key, one page for each kind of thing the keys do, so that no page is crowded.
const pages = [
  ['Moving', [
    ['Moving', [
      ['↑ ↓  j k', 'move'],
      ['PgUp PgDn', 'a page at a time'],
      ['g G', 'top / bottom'],
      ['z', 'centre the cursor'],
      ['+  -', 'open a level / close all'],
    ]],
    ['Opening', [
      ['→  l  e', 'into the fields of a track, artist, album or playlist'],
      ['enter', 'open / close; on a track, into its fields'],
      ['←  h', 'close, or up a level'],
      ['tab', 'the other pane'],
    ]],
    ['Finding', [
      ['/', 'filter  (esc clears it)'],
      ['S', 'sort the list'],
    ]],
  ]],
  ['Editing', [
    ['In the fields', [
      ['↑ ↓', 'choose a field'],
      ['enter', 'edit it; enter again saves'],
      ['esc', 'cancel, or back to the tree'],
      ['← →  1-3', "a track's tabs: details, audio, Latin forms (4: the Wolfgang, under --wolfgang)"],
      ['tab', "a track's other number"],
      ['-', 'clear a (mixed) field of an artist or album'],
    ]],
    ['Cuts, volume, fades', [
      ['[ ]', 'nudge 0.1 s or 1.5 dB'],
      ['{ }', 'nudge 1 s or 6 dB'],
      ['0', 'no cut, fade or change'],
      ['p', 'hear 8 s beside it'],
    ]],
  ]],
  ['Marking', [
    ['Several tracks', [
      ['space', 'mark a track (an album marks all its tracks)'],
      ['ctrl+a', 'mark everything shown'],
      ['e', 'edit the marked tracks together'],
    ]],
    ['Copy and paste', [
      ['c', 'copy the cover, artist, album… of a track'],
      ['v', 'paste them; ctrl+v with a picture sets the cover'],
    ]],
    ['Order and playlists', [
      ['J K', 'move a track down / up'],
      ['n', 'number an album 1…N'],
      ['p', 'add to a playlist'],
      ['d', 'remove (asks first)'],
      ['d', 'in a playlist: take a track out of it'],
    ]],
  ]],
  ['Library', [
    ['Looking up', [
      ['a', 'add from a YouTube link, or drop files on the page'],
      ['i', 'find it on MusicBrainz'],
      ['I', 'identify everything'],
      ['o', "use the file's old tags"],
    ]],
    ['Library', [
      ['s', 'sync: write the folder'],
      ['u  ctrl+r', 'undo / redo'],
      ['D', 'check the library'],
      ['?', 'this help'],
    ]],
    ['This page', [
      ['', 'Ctrl+C in the terminal where mm runs stops the program.'],
    ]],
  ]],
];

let page = 0;

export function openHelp() {
  openOverlay({
    title: 'Keys',
    render(body) {
      const tabs = h('div', { class: 'tabs', role: 'tablist' });
      pages.forEach(([name], i) => tabs.append(h('button', {
        type: 'button', role: 'tab', class: `tab${i === page ? ' on' : ''}`, 'aria-selected': String(i === page),
        onclick: () => { page = i; this.redraw(); },
      }, `${i + 1} ${name}`)));
      body.append(tabs);
      for (const [section, entries] of pages[page][1]) {
        body.append(h('h3', {}, section));
        const grid = h('div', { class: 'keys' });
        for (const [k, what] of entries) grid.append(h('div', { class: 'k' }, k), h('div', {}, what));
        body.append(grid);
      }
      body.append(h('div', { class: 'foot' }, '← → or 1-4 turn the page · esc closes'));
    },
    onKey(e) {
      const move = (d) => { page = (page + d + pages.length) % pages.length; this.redraw(); };
      if (e.key === 'Escape' || e.key === '?' || e.key === 'q' || e.key === 'Enter') closeOverlay();
      else if (e.key === 'ArrowRight' || e.key === 'l' || e.key === 'Tab') move(1);
      else if (e.key === 'ArrowLeft' || e.key === 'h') move(-1);
      else if (/^[1-9]$/.test(e.key) && Number(e.key) <= pages.length) { page = Number(e.key) - 1; this.redraw(); }
      else return false;
      return true;
    },
    hints: () => [['←→', 'page'], ['esc', 'close']],
  });
}
