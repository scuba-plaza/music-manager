import { S } from './state.js';
import { h } from './dom.js';
import { openOverlay, closeOverlay } from './overlay.js';

// openSortMenu asks how to order the library; choose(key, reverse) is called with the answer.
export function openSortMenu(choose) {
  const choices = S.lib.sort.choices;
  let cursor = Math.max(0, choices.findIndex((c) => c.key === S.lib.sort.key));
  let reverse = S.lib.sort.reverse;
  const pick = (i) => { closeOverlay(); choose(choices[i].key, choices[i].key ? reverse : false); };
  openOverlay({
    title: 'Sort',
    render(body) {
      const menu = h('div', { class: 'menu', role: 'listbox' });
      choices.forEach((c, i) => {
        const dir = c.key ? (reverse ? c.backward : c.forward) : c.forward;
        menu.append(h('div', { class: `item${i === cursor ? ' on' : ''}`, role: 'option', 'aria-selected': String(i === cursor), onclick: () => pick(i) },
          h('span', { class: 'n' }, String(i + 1)), h('span', { class: 't' }, c.label), h('span', { class: 'd' }, dir)));
      });
      body.append(menu, h('div', { class: 'foot' }, h('kbd', {}, 'r'), ` reverse: ${reverse ? 'on' : 'off'}`));
    },
    onKey(e) {
      if (e.key === 'Escape' || e.key === 'q' || e.key === 'S') closeOverlay();
      else if (e.key === 'ArrowUp' || e.key === 'k') { cursor = Math.max(cursor - 1, 0); this.redraw(); }
      else if (e.key === 'ArrowDown' || e.key === 'j') { cursor = Math.min(cursor + 1, choices.length - 1); this.redraw(); }
      else if (e.key === 'r') { reverse = !reverse; this.redraw(); }
      else if (e.key === 'Enter') pick(cursor);
      else if (/^[1-9]$/.test(e.key) && Number(e.key) <= choices.length) pick(Number(e.key) - 1);
      else return false;
      return true;
    },
    hints: () => [['enter', 'choose'], ['r', 'reverse'], ['esc', 'close']],
  });
}
