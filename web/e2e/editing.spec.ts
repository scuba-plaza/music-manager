import type { APIRequestContext, JSHandle, Locator, Page } from '@playwright/test';
import type { LibraryResponse } from '../src/api/types.gen.ts';
import { expect, test } from './fixtures.ts';

const pixel = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==';

async function openPage(page: Page): Promise<void> {
  await page.goto('/');
  await expect(page.getByRole('banner')).toContainText('6 tracks');
  await expect(page.getByRole('treeitem').first()).toBeVisible();
}

async function openAll(page: Page): Promise<void> {
  await openPage(page);
  await page.keyboard.press('+');
  await page.keyboard.press('+');
  await expect(row(page, 'So What')).toBeVisible();
}

function items(page: Page): Locator {
  return page.getByRole('tree').getByRole('treeitem');
}

function exactly(text: string): RegExp {
  return new RegExp(`^${text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}$`);
}

function row(page: Page, label: string): Locator {
  return items(page).filter({ has: page.locator('.label bdi', { hasText: exactly(label) }) });
}

async function said(page: Page, text: string): Promise<void> {
  await expect(page.getByRole('status')).toHaveText(text);
  await expect(page.getByTestId('app')).not.toHaveAttribute('aria-busy');
}

async function labelsAfter(page: Page, label: string, n: number): Promise<string[]> {
  const labels = await items(page).locator('.label').allTextContents();
  const at = labels.indexOf(label);
  return labels.slice(at + 1, at + 1 + n);
}

async function playlistCount(request: APIRequestContext, name: string): Promise<number | undefined> {
  const { view } = (await (await request.get('/api/library')).json()) as LibraryResponse;
  return view.playlists.find((p) => p.name === name)?.count;
}

async function filesOf(page: Page, files: { name: string; type: string; base64: string }[]): Promise<JSHandle<DataTransfer>> {
  return page.evaluateHandle((list) => {
    const data = new DataTransfer();
    for (const f of list) {
      data.items.add(new File([Uint8Array.from(atob(f.base64), (c) => c.charCodeAt(0))], f.name, { type: f.type }));
    }
    return data;
  }, files);
}

async function coverShown(page: Page): Promise<void> {
  const cover = page.getByRole('region', { name: 'Details' }).getByRole('img', { name: 'Cover' });
  await expect(cover).toBeVisible();
  await expect.poll(() => cover.evaluate((img: HTMLImageElement) => img.naturalWidth)).toBeGreaterThan(0);
}

test('marking two tracks and e sets the album on both with one undo step', async ({ page, request }) => {
  await openAll(page);
  await row(page, 'So What').click();
  await page.keyboard.press('Space');
  await page.keyboard.press('Space');
  await expect(page.getByRole('banner')).toContainText('2 marked');

  await page.keyboard.press('e');
  const form = page.getByRole('dialog', { name: 'Edit 2 marked tracks' });
  await expect(form.getByRole('textbox')).toHaveCount(10);
  await form.getByRole('textbox', { name: 'Album', exact: true }).fill('Kind of Blue (Legacy)');
  await expect(form.getByTestId('bulk-summary')).toHaveText('Album → Kind of Blue (Legacy)');
  await page.keyboard.press('Enter');

  await expect(form).toHaveCount(0);
  await said(page, 'Changed 2 tracks');
  await expect(row(page, 'Kind of Blue (Legacy)').locator('.right')).toHaveText('2');
  await expect(row(page, 'Kind of Blue')).toHaveCount(0);

  await page.keyboard.press('u');

  await said(page, 'Undid: bulk edit of 2 tracks');
  await expect(row(page, 'Kind of Blue').locator('.right')).toHaveText('2');
  await expect(row(page, 'Kind of Blue (Legacy)')).toHaveCount(0);
  const { history } = (await (await request.get('/api/library')).json()) as LibraryResponse;
  expect(history.undo).toBeUndefined();
});

test('the bulk form fits the window with one line for each value', async ({ page }) => {
  await openAll(page);
  await row(page, 'So What').click();
  await page.keyboard.press('Space');
  await page.keyboard.press('e');
  const form = page.getByRole('dialog', { name: 'Edit 1 marked track' });
  await expect(form).toBeVisible();

  const viewport = page.viewportSize();
  const box = await form.boundingBox();
  expect(viewport !== null && box !== null).toBe(true);
  if (viewport === null || box === null) {
    return;
  }
  expect(box.x).toBeGreaterThanOrEqual(0);
  expect(box.x + box.width).toBeLessThanOrEqual(viewport.width + 1);
  expect(box.y + box.height).toBeLessThanOrEqual(viewport.height + 1);
  const rows = await form.locator('.form .label').evaluateAll((labels) =>
    labels.map((label) => {
      const input = label.querySelector('input')?.getBoundingClientRect();
      const text = document.createRange();
      text.selectNodeContents(label.firstChild ?? label);
      const words = text.getBoundingClientRect();
      return { top: words.top, bottom: words.bottom, input: input === undefined ? null : { left: input.left, right: input.right, top: input.top, bottom: input.bottom }, wordsRight: words.right };
    }),
  );
  expect(rows).toHaveLength(10);
  let previous = -Infinity;
  for (const r of rows) {
    expect(r.input).not.toBeNull();
    if (r.input === null) {
      return;
    }
    expect(r.top).toBeLessThan(r.input.bottom);
    expect(r.bottom).toBeGreaterThan(r.input.top);
    expect(r.wordsRight).toBeLessThanOrEqual(r.input.left + 1);
    expect(r.input.right).toBeLessThanOrEqual(box.x + box.width);
    expect(r.input.top).toBeGreaterThanOrEqual(previous - 1);
    expect(Math.abs(r.input.left - (rows[0]?.input?.left ?? 0))).toBeLessThanOrEqual(1);
    previous = r.input.bottom;
  }
});

test('d asks and y removes the track and it disappears from its playlists', async ({ page, request }) => {
  await openAll(page);
  await expect(row(page, 'Blue in Green')).toHaveCount(2);
  await expect(row(page, 'Late night').locator('.right')).toHaveText('1');
  await row(page, 'Blue in Green').last().click();

  await page.keyboard.press('d');
  const ask = page.getByRole('dialog', { name: 'Are you sure?' });
  await expect(ask).toContainText('Remove “Blue in Green”?');
  await page.keyboard.press('y');

  await expect(ask).toHaveCount(0);
  await said(page, 'Removed 1 track · u undoes it');
  await expect(row(page, 'Blue in Green')).toHaveCount(0);
  await expect(row(page, 'Late night').locator('.right')).toHaveText('0');
  await expect(page.getByRole('banner')).toContainText('5 tracks');
  expect(await playlistCount(request, 'Late night')).toBe(0);
});

test('n on the question removes nothing', async ({ page }) => {
  await openAll(page);
  await row(page, 'So What').click();

  await page.keyboard.press('d');
  await expect(page.getByRole('dialog', { name: 'Are you sure?' })).toBeVisible();
  await page.keyboard.press('n');

  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(row(page, 'So What')).toHaveCount(1);
  await expect(page.getByRole('banner')).toContainText('6 tracks');
});

test('d on a track of a playlist takes it out of the playlist and leaves it in the library', async ({ page, request }) => {
  await openAll(page);
  await row(page, 'Blue in Green').first().click();

  await page.keyboard.press('d');

  await said(page, 'Took “Blue in Green” out of Late night · u undoes it');
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(row(page, 'Blue in Green')).toHaveCount(1);
  await expect(row(page, 'Late night').locator('.right')).toHaveText('0');
  await expect(page.getByRole('banner')).toContainText('6 tracks');
  expect(await playlistCount(request, 'Late night')).toBe(0);
});

test('d on a playlist asks, and y deletes it', async ({ page, request }) => {
  await openAll(page);
  await row(page, 'Late night').click();

  await page.keyboard.press('d');
  const ask = page.getByRole('dialog', { name: 'Are you sure?' });
  await expect(ask).toContainText('Delete the playlist “Late night”?');
  await expect(ask).toContainText('Its 1 track stay in the library.');
  await page.keyboard.press('Enter');

  await said(page, 'Deleted the playlist Late night · u undoes it');
  await expect(row(page, 'Late night')).toHaveCount(0);
  await expect(row(page, 'Blue in Green')).toHaveCount(1);
  expect(await playlistCount(request, 'Late night')).toBeUndefined();
});

test('J moves a track down and renumbers', async ({ page }) => {
  await openAll(page);
  await row(page, 'So What').click();
  await expect(row(page, 'So What').locator('.num')).toHaveText('01');
  await expect(row(page, 'Blue in Green').last().locator('.num')).toHaveText('03');

  await page.keyboard.press('J');

  await said(page, 'Moved');
  expect(await labelsAfter(page, 'Kind of Blue', 2)).toEqual(['Blue in Green', 'So What']);
  await expect(row(page, 'Blue in Green').last().locator('.num')).toHaveText('01');
  await expect(row(page, 'So What').locator('.num')).toHaveText('03');
  await expect(row(page, 'So What')).toHaveAttribute('aria-selected', 'true');

  await page.keyboard.press('n');

  await said(page, 'Numbered 2 tracks 1–2');
  await expect(row(page, 'Blue in Green').last().locator('.num')).toHaveText('01');
  await expect(row(page, 'So What').locator('.num')).toHaveText('02');
});

test('K on the first track of an album says it is the first already', async ({ page }) => {
  await openAll(page);
  await row(page, 'So What').click();

  await page.keyboard.press('K');

  await said(page, 'It is the first track already');
  expect(await labelsAfter(page, 'Kind of Blue', 2)).toEqual(['So What', 'Blue in Green']);
});

test('u then ctrl+r undoes and redoes', async ({ page }) => {
  await openAll(page);
  await row(page, 'So What').click();
  await page.keyboard.press('J');
  await said(page, 'Moved');

  await page.keyboard.press('u');

  await said(page, 'Undid: move of a track');
  expect(await labelsAfter(page, 'Kind of Blue', 2)).toEqual(['So What', 'Blue in Green']);
  await expect(row(page, 'So What')).toHaveAttribute('aria-selected', 'true');

  await page.keyboard.press('Control+r');

  await said(page, 'Redid: move of a track');
  expect(await labelsAfter(page, 'Kind of Blue', 2)).toEqual(['Blue in Green', 'So What']);
  await expect(row(page, 'So What')).toHaveAttribute('aria-selected', 'true');
});

test('pasting a copied cover', async ({ page }) => {
  await openAll(page);
  await row(page, 'So What').click();

  await page.keyboard.press('c');
  const list = page.getByRole('dialog', { name: 'Copy from So What' });
  await expect(list.getByRole('option', { selected: true })).toContainText('Cover');
  await page.keyboard.press('j');
  await page.keyboard.press('Space');
  await page.keyboard.press('j');
  await page.keyboard.press('Space');
  await page.keyboard.press('Enter');

  await expect(list).toHaveCount(0);
  await said(page, 'Copied the cover of So What');
  await row(page, 'Giant Steps').last().click();
  await page.keyboard.press('v');

  await said(page, 'Pasted the cover on 1 track · u undoes it');
  await page.keyboard.press('Enter');
  await coverShown(page);
  await expect(page.getByRole('region', { name: 'Details' }).getByTestId('value-artist')).toHaveText('John Coltrane');
});

test('c on a field of a track copies that field, and v pastes it on an album', async ({ page }) => {
  await openAll(page);
  await row(page, 'So What').click();
  await page.keyboard.press('Enter');
  await page.getByRole('region', { name: 'Details' }).getByTestId('value-cover').click();

  await page.keyboard.press('c');

  await said(page, 'Copied the cover of So What');
  await page.keyboard.press('Escape');
  await row(page, 'Giant Steps').nth(1).click();
  await page.keyboard.press('v');

  await said(page, 'Pasted the cover on 1 track · u undoes it');
});

test('p adds the selected album to Late night and the count becomes 3', async ({ page, request }) => {
  await openAll(page);
  await row(page, 'Kind of Blue').click();

  await page.keyboard.press('p');
  const picker = page.getByRole('dialog', { name: 'Add to a playlist' });
  await expect(picker).toContainText('Adding 2 tracks');
  await expect(picker.getByRole('option', { selected: true })).toContainText('Late night');
  await page.keyboard.press('Enter');

  await expect(picker).toHaveCount(0);
  await said(page, 'Added 1 track to Late night (1 already there) · u undoes it');
  await expect(row(page, 'Late night').locator('.right')).toHaveText('2');
  expect(await playlistCount(request, 'Late night')).toBe(2);
});

test('p then n makes a new playlist with the track', async ({ page, request }) => {
  await openAll(page);
  await row(page, 'So What').click();

  await page.keyboard.press('p');
  await page.keyboard.press('n');
  await page.getByRole('textbox', { name: 'Name of the playlist' }).fill('Road');
  await page.keyboard.press('Enter');

  await said(page, 'Made Road with 1 track · u undoes it');
  await expect(row(page, 'Road').locator('.right')).toHaveText('1');
  expect(await playlistCount(request, 'Road')).toBe(1);
});

test("renaming an artist to another artist's name says it joins them", async ({ page }) => {
  await openPage(page);
  await page.keyboard.press('+');
  await row(page, 'John Coltrane').click();
  await page.keyboard.press('ArrowRight');
  await expect(page.getByRole('region', { name: 'Details' }).getByTestId('value-artist')).toHaveText('John Coltrane');

  await page.keyboard.press('Enter');
  const name = page.getByRole('textbox', { name: 'Name' });
  await expect(name).toHaveValue('John Coltrane');
  await name.fill('Miles Davis');

  await expect(page.getByText('joins the artist Miles Davis (2 tracks)')).toBeVisible();
  await page.keyboard.press('Enter');

  await said(page, 'Changed 1 track');
  await page.keyboard.press('Escape');
  await expect(row(page, 'John Coltrane')).toHaveCount(0);
  await expect(row(page, 'Miles Davis').locator('.right')).toHaveText('3');
  await expect(row(page, 'Miles Davis')).toHaveAttribute('aria-selected', 'true');
});

test('dropping a non-image on the page is refused with a message', async ({ page }) => {
  await openAll(page);
  await row(page, 'So What').click();
  const sent: string[] = [];
  page.on('request', (r) => {
    if (r.method() !== 'GET') {
      sent.push(`${r.method()} ${r.url()}`);
    }
  });
  const files = await filesOf(page, [{ name: 'notes.txt', type: 'text/plain', base64: btoa('just words') }]);

  await page.dispatchEvent('[role="tree"]', 'dragover', { dataTransfer: files });
  await expect(page.locator('body')).toHaveClass(/dropping/);
  await page.dispatchEvent('[role="tree"]', 'drop', { dataTransfer: files });

  await said(page, 'Drop MP3 files to add them, or a JPEG or PNG picture to make it a cover');
  await expect(page.locator('body')).not.toHaveClass(/dropping/);
  expect(sent).toEqual([]);
});

test('a picture dropped on the page becomes the cover of the track under the cursor', async ({ page }) => {
  await openAll(page);
  await row(page, 'Giant Steps').last().click();
  const files = await filesOf(page, [{ name: 'front.png', type: 'image/png', base64: pixel }]);

  await page.dispatchEvent('[role="tree"]', 'drop', { dataTransfer: files });

  await said(page, 'Saved');
  await page.keyboard.press('Enter');
  await coverShown(page);
});

test('enter on the Cover field chooses a file, which becomes the cover', async ({ page }) => {
  await openAll(page);
  await row(page, 'Giant Steps').last().click();
  await page.keyboard.press('Enter');
  const pane = page.getByRole('region', { name: 'Details' });
  await pane.getByTestId('value-cover').click();
  await expect(page.getByRole('list', { name: 'Keys' })).toContainText('enter choose');

  const chooser = page.waitForEvent('filechooser');
  await page.keyboard.press('Enter');
  await (await chooser).setFiles({ name: 'front.png', mimeType: 'image/png', buffer: Buffer.from(pixel, 'base64') });

  await said(page, 'Saved');
  await coverShown(page);
});

test('- in Album turns the marked tracks into singles, and what is left blank stays', async ({ page }) => {
  await openAll(page);
  await row(page, 'Kind of Blue').click();
  await page.keyboard.press('Space');

  await page.keyboard.press('e');
  const form = page.getByRole('dialog', { name: 'Edit 2 marked tracks' });
  await form.getByRole('textbox', { name: 'Album', exact: true }).fill('-');
  await expect(form.getByTestId('bulk-summary')).toHaveText('Album → none (a single)');
  await page.keyboard.press('Enter');

  await said(page, 'Changed 2 tracks');
  await expect(row(page, 'Kind of Blue')).toHaveCount(0);
  const [group, ...singles] = await labelsAfter(page, 'Miles Davis', 3);
  expect(group).toBe('Singles');
  expect(singles.sort()).toEqual(['Blue in Green', 'So What']);
  await expect(row(page, 'Miles Davis').locator('.right')).toHaveText('2');
});

test('J in a playlist moves the track down the playlist', async ({ page }) => {
  const list = 'A rather long playlist name that goes on and on and on for a while';
  await openAll(page);
  expect(await labelsAfter(page, list, 2)).toEqual(['Giant Steps', 'وطني']);
  await row(page, 'Giant Steps').first().click();

  await page.keyboard.press('J');

  await said(page, 'Moved · u undoes it');
  expect(await labelsAfter(page, list, 2)).toEqual(['وطني', 'Giant Steps']);
  await expect(row(page, 'Giant Steps').first()).toHaveAttribute('aria-selected', 'true');
});

test('files dragged over the page outline it until they leave', async ({ page }) => {
  await openPage(page);
  const files = await filesOf(page, [{ name: 'front.png', type: 'image/png', base64: pixel }]);

  await page.dispatchEvent('[role="tree"]', 'dragover', { dataTransfer: files });
  await expect(page.locator('body')).toHaveClass(/dropping/);
  await page.dispatchEvent('[role="tree"]', 'dragleave', { dataTransfer: files });

  await expect(page.locator('body')).not.toHaveClass(/dropping/);
});
