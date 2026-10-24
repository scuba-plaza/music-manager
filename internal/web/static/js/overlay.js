import { S } from './state.js';
import { h, clear } from './dom.js';

// One dialog at a time, over the page. An overlay is an object with:
//   title            the text set into its border
//   render(body)     fills the dialog; called again by redraw()
//   onKey(e)         handles a key; returns true if it did
//   hints()          the keys the footer shows while it is open
//   afterDraw(dialog) optional: runs after each draw, to put the focus in an input
//   onClose()        optional
let lastFocus = null;

const root = () => document.getElementById('overlay-root');

export function openOverlay(o) {
  if (S.overlay) closeOverlay();
  lastFocus = document.activeElement;
  S.overlay = o;
  o.redraw = () => draw(o);
  draw(o);
  root().addEventListener('mousedown', backdrop);
}

function draw(o) {
  const dialog = h('div', { class: 'dialog', role: 'dialog', 'aria-modal': 'true', 'aria-label': o.title, tabindex: '-1' }, h('h2', {}, o.title));
  const body = h('div', { class: 'body' });
  dialog.append(body);
  o.render(body);
  clear(root()).append(dialog);
  dialog.focus({ preventScroll: true });
  if (o.afterDraw) o.afterDraw(dialog);
  document.dispatchEvent(new CustomEvent('mm:hints'));
}

function backdrop(e) {
  if (e.target === root()) closeOverlay();
}

export function closeOverlay() {
  const o = S.overlay;
  if (!o) return;
  S.overlay = null;
  root().removeEventListener('mousedown', backdrop);
  clear(root());
  if (o.onClose) o.onClose();
  if (lastFocus && lastFocus.focus && document.contains(lastFocus)) lastFocus.focus({ preventScroll: true });
  document.dispatchEvent(new CustomEvent('mm:hints'));
}
