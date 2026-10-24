// h builds an element. Text is always added as text, never as markup: names
// and titles come from files and from the internet.
export function h(tag, props, ...kids) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(props || {})) {
    if (v == null || v === false) continue;
    if (k === 'class') el.className = v;
    else if (k === 'text') el.textContent = v;
    else if (k.startsWith('on')) el.addEventListener(k.slice(2), v);
    else if (k === 'style') el.style.cssText = v;
    else el.setAttribute(k, v === true ? '' : v);
  }
  for (const kid of kids.flat(Infinity)) {
    if (kid == null || kid === false) continue;
    el.append(kid.nodeType ? kid : String(kid));
  }
  return el;
}

// name is text that may be in another direction than its surroundings (an
// Arabic title among Latin ones): it is set apart so it cannot reorder them.
export function name(text) {
  return h('bdi', { dir: 'auto' }, text);
}

export const $ = (sel, root = document) => root.querySelector(sel);

export function clear(el) {
  el.replaceChildren();
  return el;
}

export function plural(n, word) {
  if (n === 1) return `1 ${word}`;
  const es = /(ch|sh|s|x)$/.test(word) ? 'es' : 's';
  return `${n} ${word}${es}`;
}

// progress is the bar and the count of a piece of work that is done/total through.
export function progress(done, total) {
  const pct = total > 0 ? Math.min(100, Math.round((100 * done) / total)) : 0;
  return h('div', { class: 'progress-row' },
    h('div', { class: 'progress', role: 'progressbar', 'aria-valuemin': '0', 'aria-valuemax': String(total), 'aria-valuenow': String(done) }, h('i', { style: `width:${pct}%` })),
    h('span', { class: 'dim' }, `${done}/${total}`));
}
