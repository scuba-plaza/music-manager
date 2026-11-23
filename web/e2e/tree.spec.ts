import type { Page } from '@playwright/test';
import { boxOf, expect, items, library, openPage, row, test } from './fixtures.ts';

const longName = 'A rather long playlist name that goes on and on and on for a while';

function filterBox(page: Page) {
  return page.getByRole('textbox', { name: 'Filter' });
}

test('the tree lists Playlists, John Coltrane, Miles Davis and فيروز', async ({ page, request }) => {
  const { view } = await library(request);
  const topLevel = view.rows.filter((candidate) => candidate.depth === 0).map((candidate) => candidate.label);
  expect(topLevel).toEqual(expect.arrayContaining(['Playlists', 'John Coltrane', 'Miles Davis', 'فيروز']));

  await openPage(page);

  await expect(items(page)).toHaveCount(topLevel.length);
  await expect(items(page).locator('.label')).toHaveText(topLevel);
  for (const label of ['Playlists', 'John Coltrane', 'Miles Davis', 'فيروز']) {
    await expect(row(page, label)).toHaveAttribute('aria-level', '1');
  }
});

test('status:unidentified shows only the unidentified track', async ({ page }) => {
  await openPage(page);

  await filterBox(page).fill('status:unidentified');

  await expect(items(page).locator('.label')).toHaveText(['Needs work', 'Rough Mix']);
  await expect(page.getByRole('banner')).toContainText('6 tracks');
});

test('enter opens an artist and its albums, and again closes them', async ({ page }) => {
  await openPage(page);
  await row(page, 'Miles Davis').click();
  await expect(row(page, 'Miles Davis')).toHaveAttribute('aria-selected', 'true');
  await expect(row(page, 'Kind of Blue')).toHaveCount(0);

  await page.keyboard.press('Enter');

  await expect(row(page, 'Miles Davis')).toHaveAttribute('aria-expanded', 'true');
  await expect(row(page, 'Kind of Blue')).toBeVisible();

  await page.keyboard.press('Enter');

  await expect(row(page, 'Kind of Blue')).toHaveCount(0);
});

test('space marks a track and an album marks all its tracks; escape clears the marks', async ({ page }) => {
  await openPage(page);
  await row(page, 'Miles Davis').dblclick();
  await row(page, 'Kind of Blue').dblclick();
  await row(page, 'Kind of Blue').click();

  await page.keyboard.press(' ');

  await expect(row(page, 'Kind of Blue')).toHaveAttribute('data-mark', 'all');
  await expect(row(page, 'So What')).toHaveAttribute('data-mark', 'all');
  await expect(row(page, 'Blue in Green')).toHaveAttribute('data-mark', 'all');
  await expect(page.getByRole('banner')).toContainText('2 marked');

  await page.keyboard.press('Escape');

  await expect(row(page, 'So What')).not.toHaveAttribute('data-mark', /.+/);
  await expect(page.getByRole('banner')).not.toContainText('marked');
});

test('the slash key focuses the filter, enter keeps it and escape clears it, and the footer follows', async ({ page }) => {
  await openPage(page);
  const hints = page.getByRole('list', { name: 'Keys' }).getByRole('listitem');
  await expect(hints.filter({ hasText: /^\/\s*filter$/ })).toHaveText('/ filter');

  await page.keyboard.press('/');
  await expect(filterBox(page)).toBeFocused();
  await expect(hints).toHaveText(['enter keep', 'esc clear']);

  await page.keyboard.type('miles');
  await expect(items(page).locator('.label')).toHaveText(['Playlists', 'Late night', 'Blue in Green', 'Miles Davis', 'Kind of Blue', 'So What', 'Blue in Green']);
  await page.keyboard.press('Enter');
  await expect(page.getByRole('tree')).toBeFocused();
  await expect(filterBox(page)).toHaveValue('miles');
  await expect(hints.filter({ hasText: /^\/\s*filter$/ })).toHaveText('/ filter');

  await page.keyboard.press('Escape');
  await expect(filterBox(page)).toHaveValue('');
  await expect(items(page)).toHaveCount(6);
});

test('typing j, k and space in the filter box types them and marks nothing', async ({ page }) => {
  await openPage(page);
  await filterBox(page).click();

  await page.keyboard.type('jk l');

  await expect(filterBox(page)).toHaveValue('jk l');
  await expect(page.getByText('Nothing matches the filter.')).toBeVisible();
  await expect(items(page)).toHaveCount(0);
  await expect(page.getByRole('banner')).not.toContainText('marked');
});

test('nothing is asked of the server while an input method composes, and the text is asked for when it ends', async ({ page }) => {
  await openPage(page);
  const asked: string[] = [];
  page.on('request', (request) => {
    if (request.url().includes('/api/library?')) {
      asked.push(request.url());
    }
  });
  const client = await page.context().newCDPSession(page);
  await filterBox(page).focus();

  await client.send('Input.imeSetComposition', { text: 'ni', selectionStart: 2, selectionEnd: 2 });
  await client.send('Input.imeSetComposition', { text: 'nin', selectionStart: 3, selectionEnd: 3 });
  await page.waitForTimeout(400);

  expect(asked).toEqual([]);

  await client.send('Input.insertText', { text: 'nina' });

  await expect(filterBox(page)).toHaveValue('nina');
  await expect(items(page).locator('.label')).toHaveText(['Nina Simone', 'Singles', 'Feeling Good']);
  expect(asked).toHaveLength(1);
  expect(asked[0]).toContain('q=nina');
});

test('a long playlist name does not push the count out of the pane', { tag: '@narrow' }, async ({ page }) => {
  await openPage(page);
  await row(page, 'Playlists').dblclick();
  const long = row(page, longName);
  await expect(long).toBeVisible();

  const tree = await boxOf(page.getByRole('tree'));
  const count = await boxOf(long.locator('.right'));

  expect(count.x + count.width).toBeLessThanOrEqual(tree.x + tree.width);
  const overflow = await page.getByRole('tree').evaluate((element) => element.scrollWidth - element.clientWidth);
  expect(overflow).toBeLessThanOrEqual(0);
  await expect(long.locator('.right')).toHaveText('2');
});

test('the cursor row is the one that is selected and it stays inside the tree when the cursor runs to the end', { tag: '@narrow' }, async ({ page }) => {
  await page.setViewportSize({ width: page.viewportSize()?.width ?? 1200, height: 360 });
  await openPage(page);
  await page.keyboard.press('Shift+ArrowRight');
  await page.keyboard.press('Shift+ArrowRight');

  await page.keyboard.press('End');

  const last = items(page).last();
  await expect(last).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByRole('treeitem', { selected: true })).toHaveCount(1);
  const scroller = page.getByRole('tree');
  expect(await scroller.evaluate((element) => element.scrollHeight - element.clientHeight)).toBeGreaterThan(0);
  const tree = await boxOf(page.getByRole('tree'));
  const box = await boxOf(last);
  expect(box.y + box.height).toBeLessThanOrEqual(tree.y + tree.height + 1);
  expect(box.y).toBeGreaterThanOrEqual(tree.y - 1);
});
