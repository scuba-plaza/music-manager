import { S } from '/js/state.js';

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function until(ready, what) {
  for (let i = 0; i < 400; i++) {
    const value = ready();
    if (value) return value;
    await sleep(25);
  }
  throw new Error(`timed out waiting for ${what}`);
}

const box = (el) => {
  const r = el.getBoundingClientRect();
  return { x: r.left, y: r.top, w: r.width, h: r.height };
};

function lineCount(el) {
  const tops = [];
  const walk = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
  for (let node = walk.nextNode(); node; node = walk.nextNode()) {
    if (!node.textContent.trim()) continue;
    const range = document.createRange();
    range.selectNodeContents(node);
    for (const r of range.getClientRects()) tops.push(r.top);
  }
  tops.sort((a, b) => a - b);
  let lines = 0;
  let last = -Infinity;
  for (const top of tops) {
    if (top - last > 3) lines++;
    last = top;
  }
  return lines;
}

function measure() {
  const dialog = document.querySelector('.dialog');
  const menu = dialog.querySelector('.menu');
  return {
    viewport: { w: window.innerWidth, h: window.innerHeight },
    title: dialog.querySelector('h2').textContent,
    dialog: box(dialog),
    columns: getComputedStyle(menu).gridTemplateColumns.split(' ').length,
    items: [...menu.querySelectorAll('.item')].map((item) => ({
      selected: item.classList.contains('on'),
      cells: [...item.children].map((cell) => ({
        class: cell.className,
        text: cell.textContent,
        lines: lineCount(cell),
        ...box(cell),
      })),
    })),
  };
}

async function run() {
  await until(() => S.lib && document.querySelector('#tree .row'), 'the library tree');
  const row = [...document.querySelectorAll('#tree .row')].find((r) => r.dataset.key !== 'playlists');
  row.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
  await until(() => S.cursor === row.dataset.key, 'the cursor to reach the row');
  document.dispatchEvent(new KeyboardEvent('keydown', { key: 'p', bubbles: true, cancelable: true }));
  await until(() => document.querySelector('.dialog .menu .item'), 'the playlist dialog');
  await sleep(100);
  return measure();
}

const report = (payload) => fetch('/probe-result', { method: 'POST', body: JSON.stringify(payload) });

run().then(report, (err) => report({ error: String((err && err.message) || err) }));
