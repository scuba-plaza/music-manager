import type { Locator, Page } from '@playwright/test';
import type { LibraryResponse } from '../src/api/types.gen.ts';
import { helpPages } from '../src/keys/keymap.ts';
import { expect, test } from './fixtures.ts';

const longName = 'A rather long playlist name that goes on and on and on for a while';

interface Box {
  x: number;
  y: number;
  w: number;
  h: number;
}

interface Cell extends Box {
  part: string;
  text: string;
  lines: number;
}

interface Layout {
  viewport: { w: number; h: number };
  dialog: Box;
  columns: number;
  items: { selected: boolean; cells: Cell[] }[];
}

async function openPage(page: Page): Promise<void> {
  await page.goto('/');
  await expect(page.getByRole('banner')).toContainText('6 tracks');
  await expect(page.getByRole('treeitem').first()).toBeVisible();
}

function items(page: Page): Locator {
  return page.getByRole('tree').getByRole('treeitem');
}

function row(page: Page, label: string): Locator {
  return items(page).filter({ has: page.locator('.label bdi', { hasText: new RegExp(`^${label}$`) }) });
}

async function onTrack(page: Page, label: string): Promise<void> {
  await openPage(page);
  await page.keyboard.press('+');
  await page.keyboard.press('+');
  await row(page, label).last().click();
  await expect(row(page, label).last()).toHaveAttribute('aria-selected', 'true');
}

function measure(): Layout {
  const box = (el: Element): Box => {
    const r = el.getBoundingClientRect();
    return { x: r.left, y: r.top, w: r.width, h: r.height };
  };
  const lineCount = (el: Element): number => {
    const tops: number[] = [];
    const walk = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
    for (let node = walk.nextNode(); node; node = walk.nextNode()) {
      if ((node.textContent ?? '').trim() === '') {
        continue;
      }
      const range = document.createRange();
      range.selectNodeContents(node);
      for (const r of range.getClientRects()) {
        tops.push(r.top);
      }
    }
    tops.sort((a, b) => a - b);
    let lines = 0;
    let last = -Infinity;
    for (const top of tops) {
      if (top - last > 3) {
        lines += 1;
      }
      last = top;
    }
    return lines;
  };
  const dialog = document.querySelector('[role="dialog"]');
  const menu = dialog?.querySelector('[role="listbox"]');
  if (!dialog || !menu) {
    throw new Error('no dialog with a list');
  }
  return {
    viewport: { w: window.innerWidth, h: window.innerHeight },
    dialog: box(dialog),
    columns: getComputedStyle(menu).gridTemplateColumns.split(' ').length,
    items: [...menu.querySelectorAll('[role="option"]')].map((item) => ({
      selected: item.getAttribute('aria-selected') === 'true',
      cells: [...item.children].map((cell) => ({ part: cell.className, text: cell.textContent, lines: lineCount(cell), ...box(cell) })),
    })),
  };
}

function near(a: number, b: number): boolean {
  return Math.abs(a - b) <= 1;
}

test('the playlist picker rows line up', async ({ page }) => {
  await onTrack(page, 'So What');

  await page.keyboard.press('p');
  const dialog = page.getByRole('dialog', { name: 'Add to a playlist' });
  await expect(dialog.getByRole('option')).toHaveCount(3);

  const got = await page.evaluate(measure);
  const names = ['Late night', longName, 'New playlist…'];
  const counts = ['1', '2', ''];
  const d = got.dialog;

  expect(d.x).toBeGreaterThanOrEqual(0);
  expect(d.y).toBeGreaterThanOrEqual(0);
  expect(d.x + d.w).toBeLessThanOrEqual(got.viewport.w + 1);
  expect(d.y + d.h).toBeLessThanOrEqual(got.viewport.h + 1);
  expect(got.columns).toBe(3);
  expect(got.items).toHaveLength(names.length);

  const first = got.items[0];
  if (first === undefined) {
    throw new Error('no rows');
  }
  got.items.forEach((item, i) => {
    const where = `row ${String(i + 1)} (${names[i] ?? ''})`;
    expect(item.selected, where).toBe(i === 0);
    expect(item.cells, `${where} has a cell for every column`).toHaveLength(got.columns);
    item.cells.forEach((cell, c) => {
      const start = item.cells[0];
      const above = first.cells[c];
      if (start === undefined || above === undefined) {
        throw new Error(`${where}: no cell ${String(c + 1)}`);
      }
      expect(near(cell.y, start.y) && near(cell.h, start.h), `${where}: cell ${String(c + 1)} shares the top and height of the row`).toBe(true);
      expect(near(cell.x, above.x) && near(cell.w, above.w), `${where}: column ${String(c + 1)} lines up with the first row`).toBe(true);
      expect(cell.x >= d.x - 1 && cell.x + cell.w <= d.x + d.w + 1, `${where}: cell ${String(c + 1)} lies inside the dialog`).toBe(true);
      const before = item.cells[c - 1];
      if (before !== undefined) {
        expect(before.x + before.w, `${where}: cell ${String(c + 1)} does not overlap the one before it`).toBeLessThanOrEqual(cell.x + 1);
      }
    });
    const before = got.items[i - 1]?.cells[0];
    const start = item.cells[0];
    if (before !== undefined && start !== undefined) {
      expect(start.y, `${where} starts below the row above`).toBeGreaterThanOrEqual(before.y + before.h - 1);
    }
    const label = item.cells.find((cell) => cell.part === 'label');
    const count = item.cells.find((cell) => cell.part === 'detail');
    if (label === undefined || count === undefined) {
      throw new Error(`${where} has no label or no count cell`);
    }
    expect(label.text, where).toBe(names[i]);
    expect(count.text, where).toBe(counts[i]);
    expect(label.w, `${where}: the label column is at least half the dialog`).toBeGreaterThanOrEqual(d.w / 2);
    expect(count.x, `${where}: the count is right of the label`).toBeGreaterThanOrEqual(label.x + label.w - 1);
    if (i !== 1) {
      expect(label.lines, `${where} is one line`).toBe(1);
    }
    expect(count.lines, `${where}: the count is not broken`).toBeLessThanOrEqual(1);
  });
});

test('the help dialog lists every binding by page and ← → turns the pages', async ({ page }) => {
  await openPage(page);
  await page.getByRole('tree').focus();

  await page.keyboard.press('?');
  const dialog = page.getByRole('dialog', { name: 'Keys' });
  await expect(dialog).toBeVisible();
  const pages = helpPages();
  const tabs = dialog.getByRole('tab');
  await expect(tabs).toHaveText(pages.map((p, i) => `${String(i + 1)} ${p.page}`));

  for (const [i, p] of pages.entries()) {
    await expect(tabs.nth(i)).toHaveAttribute('aria-selected', 'true');
    await expect(dialog.locator('dt')).toHaveText(p.rows.map((r) => r.keys));
    await expect(dialog.locator('dd')).toHaveText(p.rows.map((r) => r.text));
    await page.keyboard.press('ArrowRight');
  }
  await expect(tabs.nth(0)).toHaveAttribute('aria-selected', 'true');

  await page.keyboard.press('ArrowLeft');
  await expect(tabs.nth(pages.length - 1)).toHaveAttribute('aria-selected', 'true');
  await page.keyboard.press('ArrowLeft');
  await expect(tabs.nth(pages.length - 2)).toHaveAttribute('aria-selected', 'true');
  await expect(dialog).toContainText('Ctrl+C in the terminal where mm runs stops the program.');

  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
});

test('S then 3 sorts by title into one flat list and choosing the tree goes back to the tree', async ({ page, request }) => {
  const sorted = (await (await request.get('/api/library?sort=title')).json()) as LibraryResponse;
  const titles = sorted.view.rows.map((r) => r.label);
  await openPage(page);
  await page.getByRole('tree').focus();

  await page.keyboard.press('S');
  const dialog = page.getByRole('dialog', { name: 'Sort' });
  await expect(dialog.getByRole('option').nth(3)).toContainText('Title');
  await page.keyboard.press('4');

  await expect(dialog).toHaveCount(0);
  await expect(items(page).locator('.label')).toHaveText(titles);
  for (const level of await items(page).evaluateAll((rows) => rows.map((r) => r.getAttribute('aria-level')))) {
    expect(level).toBe('1');
  }
  await row(page, 'So What').click();
  await expect(row(page, 'So What')).toHaveAttribute('aria-selected', 'true');

  await page.keyboard.press('S');
  await page.keyboard.press('1');

  await expect(dialog).toHaveCount(0);
  await expect(row(page, 'Playlists')).toHaveAttribute('aria-level', '1');
  await expect(row(page, 'So What').last()).toHaveAttribute('aria-selected', 'true');
  await expect(row(page, 'So What').last()).toHaveAttribute('aria-level', '3');
});
