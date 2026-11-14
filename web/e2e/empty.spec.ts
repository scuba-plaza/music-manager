import type { APIRequestContext, Locator, Page } from '@playwright/test';
import type { LibraryResponse } from '../src/api/types.gen.ts';
import { expect, test } from './fixtures.ts';

const headers = { 'X-MM': '1' };

async function library(request: APIRequestContext): Promise<LibraryResponse> {
  return (await (await request.get('/api/library')).json()) as LibraryResponse;
}

async function removeEverything(request: APIRequestContext): Promise<void> {
  const ids = (await library(request)).view.rows.flatMap((r) => (r.trackId === undefined ? [] : [r.trackId]));
  const res = await request.post('/api/remove', { headers, data: { ids: [...new Set(ids)] } });
  expect(res.ok()).toBe(true);
}

async function removePlaylists(request: APIRequestContext): Promise<void> {
  for (const p of (await library(request)).view.playlists) {
    const res = await request.post('/api/playlists/delete', { headers, data: { name: p.name } });
    expect(res.ok()).toBe(true);
  }
}

function row(page: Page, label: string): Locator {
  const exact = new RegExp(`^${label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}$`);
  return page
    .getByRole('tree')
    .getByRole('treeitem')
    .filter({ has: page.locator('.label bdi', { hasText: exact }) });
}

test('an empty library shows an empty tree and keys do nothing harmful', async ({ page, request }) => {
  await removePlaylists(request);
  await removeEverything(request);
  await page.goto('/');
  await expect(page.getByRole('banner')).toContainText('0 tracks');

  await expect(page.getByRole('tree').getByRole('treeitem')).toHaveCount(0);
  const errors: string[] = [];
  page.on('pageerror', (err) => errors.push(err.message));
  await page.getByRole('tree').focus();
  for (const key of ['ArrowDown', 'ArrowUp', 'Enter', 'ArrowRight', 'ArrowLeft', '+', '-', 'z', 'Space', 'Control+a', 'e', 'd', 'J', 'K', 'n', 'c', 'v', 'i', 'o', 'p']) {
    await page.keyboard.press(key);
    await page.keyboard.press('Escape');
  }

  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(page.getByRole('tree').getByRole('treeitem')).toHaveCount(0);
  await expect(page.getByRole('banner')).toContainText('0 tracks');
  expect(errors).toEqual([]);
  await page.keyboard.press('?');
  await expect(page.getByRole('dialog')).toHaveAccessibleName('Keys');
});

test('a library without playlists makes p open on the name input', async ({ page, request }) => {
  await removePlaylists(request);
  await page.goto('/');
  await expect(page.getByRole('banner')).toContainText('6 tracks');
  await page.keyboard.press('+');
  await page.keyboard.press('+');
  await row(page, 'So What').last().click();

  await page.keyboard.press('p');

  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible();
  const name = dialog.getByRole('textbox');
  await expect(name).toBeFocused();
  await expect(dialog.getByRole('option')).toHaveCount(0);
});

test('an unidentified track shows its fields and the Needs work group lists it', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('banner')).toContainText('1 need work');
  await page.keyboard.press('+');
  await page.keyboard.press('+');
  await row(page, 'Needs work').first().click();
  const pane = page.getByRole('region', { name: 'Details' });

  await row(page, 'Rough Mix').last().click();
  await page.keyboard.press('Enter');

  await expect(pane.getByRole('heading', { level: 3 })).toHaveText('Rough Mix');
  await expect(pane.getByTestId('value-title')).toBeVisible();
  await expect(pane.getByTestId('value-artist')).toBeVisible();
});

test('an empty playlist opens with no rows and d deletes it after asking', async ({ page, request }) => {
  const lib = await library(request);
  const blue = lib.view.rows.find((r) => r.kind === 'track' && r.label === 'Blue in Green')?.trackId;
  const took = await request.post('/api/playlists/takeout', { headers, data: { name: 'Late night', ids: [blue] } });
  expect(took.ok()).toBe(true);
  await page.goto('/');
  await expect(page.getByRole('banner')).toContainText('6 tracks');
  await page.keyboard.press('+');
  await row(page, 'Late night').first().click();
  await page.keyboard.press('Enter');
  await expect(row(page, 'Late night').first()).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByRole('tree').getByRole('treeitem').filter({ has: page.locator('[class~="track"]') }).filter({ hasText: 'Blue in Green' })).toHaveCount(0);

  await page.keyboard.press('d');
  await expect(page.getByRole('dialog')).toContainText('Late night');
  await page.keyboard.press('y');

  await expect(row(page, 'Late night')).toHaveCount(0);
  await expect.poll(async () => (await library(request)).view.playlists.map((p) => p.name)).not.toContain('Late night');
});
