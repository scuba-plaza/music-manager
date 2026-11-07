import type { Locator, Page } from '@playwright/test';
import type { LibraryResponse } from '../src/api/types.gen.ts';
import { expect, test } from './fixtures.ts';

async function openAll(page: Page): Promise<void> {
  await page.goto('/');
  await expect(page.getByRole('banner')).toContainText('6 tracks');
  await page.keyboard.press('+');
  await page.keyboard.press('+');
  await expect(row(page, 'So What').first()).toBeVisible();
}

function row(page: Page, label: string): Locator {
  const exact = new RegExp(`^${label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}$`);
  return page
    .getByRole('tree')
    .getByRole('treeitem')
    .filter({ has: page.locator('.label bdi', { hasText: exact }) });
}

async function box(target: Locator): Promise<{ x: number; width: number }> {
  const found = await target.boundingBox();
  if (found === null) {
    throw new Error('the element has no box');
  }
  return found;
}

test('Arabic rows keep dir=auto and the count column does not move', async ({ page }) => {
  await openAll(page);

  for (const [arabic, latin] of [
    ['فيروز', 'John Coltrane'],
    ['Songs', 'Giant Steps'],
  ] as const) {
    const a = row(page, arabic).first();
    const l = row(page, latin).first();
    await expect(a.locator('bdi').first()).toHaveAttribute('dir', 'auto');
    const countX = async (target: Locator) => (await box(target.locator('.right'))).x + (await box(target.locator('.right'))).width;
    expect(await countX(a)).toBeCloseTo(await countX(l), 0);
  }
  const arabicTrack = row(page, 'وطني').last();
  const latinTrack = row(page, 'So What').last();
  await expect(arabicTrack.locator('.label bdi')).toHaveAttribute('dir', 'auto');
  expect((await box(arabicTrack.locator('.label'))).x).toBeCloseTo((await box(latinTrack.locator('.label'))).x, 0);
  const right = async (target: Locator) => (await box(target.locator('.right'))).x + (await box(target.locator('.right'))).width;
  expect(await right(arabicTrack)).toBeCloseTo(await right(latinTrack), 0);
});

test('sorting by artist puts فيروز among the F, by its Latin form', async ({ page, request }) => {
  const sorted = (await (await request.get('/api/library?sort=artist')).json()) as LibraryResponse;
  const artists = sorted.view.rows.map((r) => r.detail ?? '');
  expect(artists.slice(0, 3)).toEqual(['Basement Tapes', 'فيروز', 'John Coltrane']);
  await page.goto('/');
  await expect(page.getByRole('banner')).toContainText('6 tracks');
  await page.getByRole('tree').focus();

  await page.keyboard.press('S');
  await page.keyboard.press('2');

  const labels = page.getByRole('tree').getByRole('treeitem').locator('.label');
  await expect(labels.nth(1)).toHaveText('وطني');
  await expect(labels).toHaveText(sorted.view.rows.map((r) => r.label));
});

test('a mixed Latin and Arabic title does not reorder its neighbours', async ({ page, request }) => {
  const { view } = (await (await request.get('/api/library')).json()) as LibraryResponse;
  const giant = view.rows.find((r) => r.kind === 'track' && r.label === 'Giant Steps');
  const title = 'Hello وطني world';
  const patched = await request.patch(`/api/tracks/${giant?.trackId ?? ''}`, { headers: { 'X-MM': '1' }, data: { field: 'title', text: title } });
  expect(patched.ok()).toBe(true);
  await openAll(page);

  const mixed = row(page, title).last();
  await expect(mixed.locator('.label bdi')).toHaveText(title);
  await expect(mixed.locator('.label bdi')).toHaveAttribute('dir', 'auto');
  expect(await mixed.locator('.label bdi').evaluate((el) => el.textContent)).toBe(title);
  const neighbour = row(page, 'So What').last();
  expect((await box(mixed.locator('.label'))).x).toBeCloseTo((await box(neighbour.locator('.label'))).x, 0);
  const right = async (target: Locator) => (await box(target.locator('.right'))).x + (await box(target.locator('.right'))).width;
  expect(await right(mixed)).toBeCloseTo(await right(neighbour), 0);
});

test('the Latin tab and the Device tab show the Latin form for the Arabic track', async ({ page }) => {
  await openAll(page);
  await row(page, 'وطني').last().click();
  await page.keyboard.press('Enter');
  const pane = page.getByRole('region', { name: 'Details' });
  await expect(pane.getByRole('heading', { level: 3 })).toHaveText('وطني');

  await page.keyboard.press('3');
  await expect(pane.getByTestId('value-latinTitle')).toHaveText('Watani');

  await pane.locator('[data-tab="device"]').click();
  await expect(pane.getByText('01 - Watani.mp3')).toBeVisible();
});
