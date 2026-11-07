import type { JSHandle, Locator, Page } from '@playwright/test';
import type { LibraryResponse } from '../src/api/types.gen.ts';
import { expect, test } from './fixtures.ts';

const pixel = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==';

async function openAll(page: Page): Promise<void> {
  await page.goto('/');
  await expect(page.getByRole('banner')).toContainText('6 tracks');
  await page.keyboard.press('+');
  await page.keyboard.press('+');
  await expect(row(page, 'So What')).toBeVisible();
}

function row(page: Page, label: string): Locator {
  const exact = new RegExp(`^${label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}$`);
  return page
    .getByRole('tree')
    .getByRole('treeitem')
    .filter({ has: page.locator('.label bdi', { hasText: exact }) });
}

function pane(page: Page): Locator {
  return page.getByRole('region', { name: 'Details' });
}

async function intoTrack(page: Page, label: string): Promise<void> {
  await row(page, label).last().click();
  await page.keyboard.press('Enter');
  await expect(pane(page).getByRole('heading', { level: 3 })).toHaveText(label);
}

async function said(page: Page, text: string | RegExp): Promise<void> {
  await expect(page.getByRole('status')).toHaveText(text);
  await expect(page.getByTestId('app')).not.toHaveAttribute('aria-busy');
}

async function clipboardOf(page: Page, name: string): Promise<JSHandle<DataTransfer>> {
  return page.evaluateHandle(
    ([file, data]) => {
      const transfer = new DataTransfer();
      transfer.items.add(new File([Uint8Array.from(atob(data ?? ''), (c) => c.charCodeAt(0))], file ?? '', { type: 'image/png' }));
      return transfer;
    },
    [name, pixel],
  );
}

test('] nudges a cut by a second, [ by a tenth, { and } by ten, and 0 clears it', async ({ page }) => {
  await openAll(page);
  await intoTrack(page, 'So What');
  await page.keyboard.press('2');
  await pane(page).getByTestId('value-cutStart').click();
  await page.keyboard.press('Escape');
  const value = pane(page).getByTestId('value-cutStart');
  const blank = (await value.textContent()) ?? '';

  await page.keyboard.press(']');
  await expect(value).not.toHaveText(blank);
  await expect(value).toContainText('1');
  const second = (await value.textContent()) ?? '';

  await page.keyboard.press('[');
  await expect(value).not.toHaveText(second);
  const tenth = (await value.textContent()) ?? '';

  await page.keyboard.press('}');
  await expect(value).not.toHaveText(tenth);
  const ten = (await value.textContent()) ?? '';

  await page.keyboard.press('{');
  await expect(value).not.toHaveText(ten);
  await expect(value).toHaveText(tenth);

  await page.keyboard.press('0');
  await expect(value).toHaveText(blank);
});

test('n numbers the album and the tracks follow', async ({ page, request }) => {
  await openAll(page);
  await row(page, 'Kind of Blue').first().click();

  await page.keyboard.press('n');

  await expect(page.getByRole('status')).not.toHaveText('');
  const { view } = (await (await request.get('/api/library')).json()) as LibraryResponse;
  const numbers = view.rows.filter((r) => r.parent === 'album\u001fMiles Davis\u001fKind of Blue').map((r) => r.num);
  expect(numbers).toEqual(['01', '02']);
});

test('x on the Cover field removes the cover of the track', async ({ page }) => {
  await openAll(page);
  await intoTrack(page, 'So What');
  await pane(page).getByTestId('value-cover').click();
  await expect(pane(page).getByRole('img', { name: 'Cover' })).toBeVisible();

  await page.keyboard.press('x');

  await said(page, 'Saved');
  await expect(pane(page).getByRole('img', { name: 'Cover' })).toHaveCount(0);
});

test('a picture pasted with ctrl+v becomes the cover of the track under the cursor', async ({ page }) => {
  await openAll(page);
  await row(page, 'Giant Steps').last().click();
  const data = await clipboardOf(page, 'image.png');

  await page.evaluate((transfer) => {
    document.dispatchEvent(new ClipboardEvent('paste', { clipboardData: transfer, bubbles: true, cancelable: true }));
  }, data);

  await said(page, /^Saved|cover/);
  await page.keyboard.press('Enter');
  const cover = pane(page).getByRole('img', { name: 'Cover' });
  await expect(cover).toBeVisible();
  await expect.poll(() => cover.evaluate((img: HTMLImageElement) => img.naturalWidth)).toBeGreaterThan(0);
});

test('what another program changes shows on the page within a few seconds', async ({ page, request }) => {
  await openAll(page);
  const { view } = (await (await request.get('/api/library')).json()) as LibraryResponse;
  const giant = view.rows.find((r) => r.kind === 'track' && r.label === 'Giant Steps');

  const patched = await request.patch(`/api/tracks/${giant?.trackId ?? ''}`, { headers: { 'X-MM': '1' }, data: { field: 'title', text: 'Giant Strides' } });
  expect(patched.ok()).toBe(true);

  await expect(row(page, 'Giant Strides').first()).toBeVisible({ timeout: 8000 });
});

test('click selects, double-click opens, click on a field then again edits, the twist arrow opens at once', async ({ page }) => {
  await openAll(page);
  await page.keyboard.press('-');
  await expect(row(page, 'Miles Davis').first()).toBeVisible();

  await row(page, 'Miles Davis').first().click();
  await expect(row(page, 'Miles Davis').first()).toHaveAttribute('aria-selected', 'true');
  await expect(row(page, 'Kind of Blue')).toHaveCount(0);

  await row(page, 'Miles Davis').first().dblclick();
  await expect(row(page, 'Kind of Blue')).toBeVisible();

  await row(page, 'Kind of Blue').first().locator('.twist').click();
  await expect(row(page, 'So What').first()).toBeVisible();
  await row(page, 'Kind of Blue').first().locator('.twist').click();
  await expect(row(page, 'So What')).toHaveCount(0);
  await row(page, 'Kind of Blue').first().locator('.twist').click();
  await expect(row(page, 'So What').first()).toBeVisible();

  await row(page, 'So What').last().dblclick();
  await expect(pane(page).getByRole('heading', { level: 3 })).toHaveText('So What');
  await pane(page).getByTestId('value-artist').click();
  await expect(pane(page).getByRole('textbox', { name: 'Artist' })).toHaveCount(0);
  await pane(page).getByTestId('value-artist').click();
  await expect(pane(page).getByRole('textbox', { name: 'Artist' })).toBeFocused();
});

test('the tab you are on stays when another track is selected', async ({ page }) => {
  await openAll(page);
  await intoTrack(page, 'So What');
  await page.keyboard.press('2');
  const tab = pane(page).getByRole('tab', { selected: true });
  const name = await tab.textContent();

  await page.keyboard.press('Escape');
  await row(page, 'Giant Steps').last().click();
  await page.keyboard.press('Enter');

  await expect(pane(page).getByRole('tab', { selected: true })).toHaveText(name ?? '');
});
