import type { APIRequestContext } from '@playwright/test';
import { expect, headers, items, library, openAll, row, test } from './fixtures.ts';

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

test('an empty library shows an empty tree and keys do nothing harmful', async ({ page, request }) => {
  await removePlaylists(request);
  await removeEverything(request);
  await page.goto('/');
  await expect(page.getByRole('banner')).toContainText('0 tracks');

  await expect(items(page)).toHaveCount(0);
  const errors: string[] = [];
  page.on('pageerror', (err) => errors.push(err.message));
  await page.getByRole('tree').focus();
  for (const key of ['ArrowDown', 'ArrowUp', 'Enter', 'ArrowRight', 'ArrowLeft', 'Shift+ArrowRight', 'Shift+ArrowLeft', 'Space', 'Shift+Space', 'e', 'd', 'Shift+ArrowDown', 'Shift+ArrowUp', 'n', 'c', 'v', 'p']) {
    await page.keyboard.press(key);
    await page.keyboard.press('Escape');
  }

  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(items(page)).toHaveCount(0);
  await expect(page.getByRole('banner')).toContainText('0 tracks');
  expect(errors).toEqual([]);
  await page.keyboard.press('?');
  await expect(page.getByRole('dialog')).toHaveAccessibleName('Keys');
});

test('a library without playlists makes p open on the name input', async ({ page, dialog, request }) => {
  await removePlaylists(request);
  await openAll(page);
  await row(page, 'So What').click();

  await page.keyboard.press('p');

  await expect(dialog()).toBeVisible();
  await expect(dialog().getByRole('textbox')).toBeFocused();
  await expect(dialog().getByRole('option')).toHaveCount(0);
});
