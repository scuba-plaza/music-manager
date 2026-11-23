import type { Locator } from '@playwright/test';
import type { LibraryResponse } from '../src/api/types.gen.ts';
import { boxOf, expect, headers, library, openAll, openPage, row, test } from './fixtures.ts';

async function labelStart(target: Locator): Promise<number> {
  return (await boxOf(target.locator('.label'))).x;
}

async function countEnd(target: Locator): Promise<number> {
  const box = await boxOf(target.locator('.right'));
  return box.x + box.width;
}

test('Arabic rows keep dir=auto, start their labels where the Latin ones do and keep the counts inside the tree', { tag: '@narrow' }, async ({ page }) => {
  await openAll(page);
  const tree = await boxOf(page.getByRole('tree'));

  for (const [arabic, latin] of [
    ['فيروز', 'John Coltrane'],
    ['Songs', 'Kind of Blue'],
    ['وطني', 'So What'],
  ] as const) {
    const a = row(page, arabic).last();
    const l = row(page, latin).last();
    await expect(a.locator('.label bdi')).toHaveAttribute('dir', 'auto');
    expect(await labelStart(a)).toBeCloseTo(await labelStart(l), 0);
    expect(await countEnd(a)).toBeCloseTo(await countEnd(l), 0);
    expect(await countEnd(a)).toBeLessThanOrEqual(tree.x + tree.width);
  }
});

test('sorting by artist puts فيروز among the F, by its Latin form', async ({ page, request }) => {
  const sorted = (await (await request.get('/api/library?sort=artist')).json()) as LibraryResponse;
  const artists = sorted.view.rows.map((r) => r.detail ?? '');
  expect(artists.slice(0, 3)).toEqual(['Basement Tapes', 'فيروز', 'John Coltrane']);
  await openPage(page);
  await page.getByRole('tree').focus();

  await page.keyboard.press('o');
  await page.keyboard.press('2');

  const labels = page.getByRole('tree').getByRole('treeitem').locator('.label');
  await expect(labels.nth(1)).toHaveText('وطني');
  await expect(labels).toHaveText(sorted.view.rows.map((r) => r.label));
});

test('a mixed Latin and Arabic title does not reorder its neighbours', { tag: '@narrow' }, async ({ page, request }) => {
  const giant = (await library(request)).view.rows.find((r) => r.kind === 'track' && r.label === 'Giant Steps');
  const title = 'Hello وطني world';
  const patched = await request.patch(`/api/tracks/${giant?.trackId ?? ''}`, { headers, data: { field: 'title', text: title } });
  expect(patched.ok()).toBe(true);
  await openAll(page);

  const mixed = row(page, title).last();
  await expect(mixed.locator('.label bdi')).toHaveText(title);
  await expect(mixed.locator('.label bdi')).toHaveAttribute('dir', 'auto');
  const neighbour = row(page, 'So What').last();
  expect(await labelStart(mixed)).toBeCloseTo(await labelStart(neighbour), 0);
  expect(await countEnd(mixed)).toBeCloseTo(await countEnd(neighbour), 0);
});
