import type { Locator, Page } from '@playwright/test';
import type { LibraryResponse } from '../src/api/types.gen.ts';
import { expect, test } from './fixtures.ts';

async function openPage(page: Page): Promise<void> {
  await page.goto('/');
  await expect(page.getByRole('banner')).toContainText('6 tracks');
  await expect(page.getByRole('treeitem').first()).toBeVisible();
}

function items(page: Page): Locator {
  return page.getByRole('tree').getByRole('treeitem');
}

function row(page: Page, label: string): Locator {
  return items(page).filter({ has: page.locator('.label bdi', { hasText: label }) });
}

function filterBox(page: Page): Locator {
  return page.getByRole('textbox', { name: 'Filter' });
}

async function boxOf(locator: Locator): Promise<{ x: number; y: number; width: number; height: number }> {
  const box = await locator.boundingBox();
  if (box === null) {
    throw new Error('the element has no box on the page');
  }
  return box;
}

test('the tree lists Playlists, John Coltrane, Miles Davis and فيروز', async ({ page, request }) => {
  const { view } = (await (await request.get('/api/library')).json()) as LibraryResponse;
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

test('the Arabic artist row is a bdi with dir=auto', async ({ page }) => {
  await openPage(page);

  const name = row(page, 'فيروز').locator('bdi');

  await expect(name).toHaveText('فيروز');
  await expect(name).toHaveAttribute('dir', 'auto');
  await expect(name).toHaveCSS('unicode-bidi', 'isolate');
  await expect(row(page, 'John Coltrane').locator('bdi')).toHaveAttribute('dir', 'auto');
});

test('an Arabic name does not move its neighbours: the labels start at one edge and the counts end at the other', async ({ page }) => {
  await openPage(page);
  const arabic = row(page, 'فيروز');
  const latin = row(page, 'John Coltrane');

  const labelStart = async (target: Locator) => (await boxOf(target.locator('.label'))).x;
  const countEnd = async (target: Locator) => {
    const box = await boxOf(target.locator('.right'));
    return box.x + box.width;
  };

  expect(await labelStart(arabic)).toBeCloseTo(await labelStart(latin), 0);
  expect(await countEnd(arabic)).toBeCloseTo(await countEnd(latin), 0);
  const tree = await boxOf(page.getByRole('tree'));
  expect(await countEnd(arabic)).toBeLessThanOrEqual(tree.x + tree.width);
});

test('enter opens an artist and its albums, and again closes them', async ({ page, press }) => {
  await openPage(page);
  await row(page, 'Miles Davis').click();
  await expect(row(page, 'Miles Davis')).toHaveAttribute('aria-selected', 'true');
  await expect(row(page, 'Kind of Blue')).toHaveCount(0);

  await press('Enter');

  await expect(row(page, 'Miles Davis')).toHaveAttribute('aria-expanded', 'true');
  await expect(row(page, 'Kind of Blue')).toBeVisible();

  await press('Enter');

  await expect(row(page, 'Kind of Blue')).toHaveCount(0);
});

test('+ opens a level and - closes all of it', async ({ page, press }) => {
  await openPage(page);
  await row(page, 'Miles Davis').click();

  await press('+');
  await expect(row(page, 'Kind of Blue')).toBeVisible();
  await expect(row(page, 'Songs')).toBeVisible();
  await expect(row(page, 'Rough Mix')).toBeVisible();
  await expect(row(page, 'So What')).toHaveCount(0);

  await press('=');
  await expect(row(page, 'So What')).toBeVisible();
  await expect(row(page, 'وطني').first()).toBeVisible();

  await press('-');
  await expect(items(page)).toHaveCount(6);
});

test('a double click opens a row and a click on the arrow opens another at once', async ({ page }) => {
  await openPage(page);

  await row(page, 'Miles Davis').dblclick();
  await expect(row(page, 'Kind of Blue')).toBeVisible();

  await row(page, 'John Coltrane').locator('.twist').click();
  await expect(row(page, 'John Coltrane')).toHaveAttribute('aria-expanded', 'true');
  await expect(row(page, 'John Coltrane')).toHaveAttribute('aria-selected', 'true');
});

test('space marks a track and an album marks all its tracks; escape clears the marks', async ({ page, press }) => {
  await openPage(page);
  await row(page, 'Miles Davis').dblclick();
  await row(page, 'Kind of Blue').dblclick();
  await row(page, 'Kind of Blue').click();

  await press(' ');

  await expect(row(page, 'Kind of Blue')).toHaveAttribute('data-mark', 'all');
  await expect(row(page, 'So What')).toHaveAttribute('data-mark', 'all');
  await expect(row(page, 'Blue in Green')).toHaveAttribute('data-mark', 'all');
  await expect(page.getByRole('banner')).toContainText('2 marked');

  await press('Escape');

  await expect(row(page, 'So What')).not.toHaveAttribute('data-mark', /.+/);
  await expect(page.getByRole('banner')).not.toContainText('marked');
});

test('ctrl+a marks every track that is shown', async ({ page, press }) => {
  await openPage(page);
  await row(page, 'Needs work').dblclick();
  await row(page, 'Miles Davis').dblclick();
  await row(page, 'Kind of Blue').dblclick();

  await press('Control+a');

  await expect(page.getByRole('banner')).toContainText('3 marked');
  await expect(row(page, 'Rough Mix')).toHaveAttribute('data-mark', 'all');
  await expect(row(page, 'So What')).toHaveAttribute('data-mark', 'all');
  await expect(row(page, 'John Coltrane')).not.toHaveAttribute('data-mark', /.+/);
});

test('the slash key focuses the filter, enter keeps it and escape clears it, and the footer follows', async ({ page, press }) => {
  await openPage(page);
  const hints = page.getByRole('list', { name: 'Keys' }).getByRole('listitem');
  await expect(hints.filter({ hasText: /^\/\s*filter$/ })).toHaveText('/ filter');

  await press('/');
  await expect(filterBox(page)).toBeFocused();
  await expect(hints).toHaveText(['enter keep', 'esc clear']);

  await page.keyboard.type('miles');
  await expect(items(page).locator('.label')).toHaveText(['Playlists', 'Late night', 'Blue in Green', 'Miles Davis', 'Kind of Blue', 'So What', 'Blue in Green']);
  await press('Enter');
  await expect(page.getByRole('tree')).toBeFocused();
  await expect(filterBox(page)).toHaveValue('miles');
  await expect(hints.filter({ hasText: /^\/\s*filter$/ })).toHaveText('/ filter');

  await press('Escape');
  await expect(filterBox(page)).toHaveValue('');
  await expect(items(page)).toHaveCount(6);
});

test('escape in the filter box clears it and leaves the box', async ({ page, press }) => {
  await openPage(page);
  await filterBox(page).fill('nina');
  await expect(items(page).locator('.label')).toHaveText(['Nina Simone', 'Singles', 'Feeling Good']);

  await press('Escape');

  await expect(filterBox(page)).toHaveValue('');
  await expect(filterBox(page)).not.toBeFocused();
  await expect(items(page)).toHaveCount(6);
});

test('typing j, k and space in the filter box types them and marks nothing', async ({ page }) => {
  await openPage(page);
  await filterBox(page).click();

  await page.keyboard.type('jk l');

  await expect(filterBox(page)).toHaveValue('jk l');
  await expect(page.getByText('Nothing matches the filter.')).toBeVisible();
  await expect(page.getByRole('banner')).not.toContainText('marked');
});

test('a filter that matches nothing says so', async ({ page }) => {
  await openPage(page);

  await filterBox(page).fill('genre:folk');

  await expect(page.getByText('Nothing matches the filter.')).toBeVisible();
  await expect(items(page)).toHaveCount(0);
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

test('a long playlist name does not push the count out of the pane', async ({ page }) => {
  await openPage(page);
  await row(page, 'Playlists').dblclick();
  const long = row(page, 'A rather long playlist name');
  await expect(long).toBeVisible();

  const tree = await boxOf(page.getByRole('tree'));
  const count = await boxOf(long.locator('.right'));

  expect(count.x + count.width).toBeLessThanOrEqual(tree.x + tree.width);
  const overflow = await page.getByRole('tree').evaluate((element) => element.scrollWidth - element.clientWidth);
  expect(overflow).toBeLessThanOrEqual(0);
  await expect(long.locator('.right')).toHaveText('2');
});

test('the cursor row is the one that is selected and it stays inside the tree when the cursor runs to the end', async ({ page, press }) => {
  await page.setViewportSize({ width: page.viewportSize()?.width ?? 1200, height: 360 });
  await openPage(page);
  await press('+');
  await press('=');

  await press('End');

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
