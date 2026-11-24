import type { TrackView } from '../src/api/types.gen.ts';
import { expect, headers, library, openPage, row, said, test } from './fixtures.ts';

const playlistLink = 'https://music.youtube.com/playlist?list=OLAK5uy_kindofblue';
const tooLongLink = 'https://music.youtube.com/playlist?list=toolong';

test('a adds a playlist as an album with the tracks the person left ticked', async ({ page, dialog, request }) => {
  await openPage(page);

  await page.keyboard.press('a');
  await dialog().getByRole('textbox', { name: 'Link' }).fill(playlistLink);
  await page.keyboard.press('Enter');

  await expect(dialog().getByRole('textbox', { name: 'Album' })).toHaveValue('Kind of Blue');
  await expect(dialog().getByRole('textbox', { name: 'Artist' })).toHaveValue('Miles Davis');
  await expect(dialog().getByRole('checkbox')).toHaveCount(5);
  await expect(dialog().getByRole('checkbox', { checked: true })).toHaveCount(5);
  const cover = dialog().getByRole('img', { name: 'Cover' });
  await expect(cover).toBeVisible();
  await expect.poll(() => cover.evaluate((img: HTMLImageElement) => img.naturalWidth)).toBeGreaterThan(0);

  await dialog().getByRole('checkbox', { name: '2. Freddie Freeloader' }).uncheck();
  await page.keyboard.press('Enter');

  await expect(dialog()).toHaveCount(0);
  await said(page, 'Added 4 tracks of "Kind of Blue" · u undoes it');
  await expect(page.getByRole('banner')).toContainText('10 tracks');
  await page.keyboard.press('Shift+ArrowRight');
  await page.keyboard.press('Shift+ArrowRight');
  await expect(row(page, 'So What')).toHaveCount(2);
  await expect(row(page, 'Freddie Freeloader')).toHaveCount(0);
  await expect(row(page, 'All Blues').locator('.num')).toHaveText('04');
  await expect(row(page, 'Flamenco Sketches').locator('.num')).toHaveText('05');
  const added = (await library(request)).view.rows.filter((r) => ['So What', 'Blue in Green', 'All Blues', 'Flamenco Sketches'].includes(r.label) && (r.parent ?? '').startsWith('album'));
  expect(added).toHaveLength(6);
  const numbers: string[] = [];
  for (const r of added) {
    const track = (await (await request.get(`/api/tracks/${r.trackId ?? ''}`, { headers })).json()) as TrackView;
    const fields = track.tabs.flatMap((t) => t.fields);
    numbers.push(fields.find((f) => f.key === 'track')?.text ?? '');
  }
  expect(numbers.filter((n) => n.endsWith(' of 5')).sort()).toEqual(['1 of 5', '3 of 5', '4 of 5', '5 of 5']);

  await page.keyboard.press('u');
  await expect(page.getByRole('banner')).toContainText('6 tracks');
});

test('a playlist with nothing ticked cannot be added', async ({ page, dialog }) => {
  await openPage(page);
  await page.keyboard.press('a');
  await dialog().getByRole('textbox', { name: 'Link' }).fill(playlistLink);
  await page.keyboard.press('Enter');
  await expect(dialog().getByRole('checkbox')).toHaveCount(5);

  await dialog().getByRole('button', { name: 'Select none' }).click();
  await dialog().getByRole('textbox', { name: 'Album' }).press('Enter');

  await expect(dialog()).toContainText('Pick at least one track');
  await expect(dialog()).toBeVisible();
  await expect(page.getByRole('banner')).toContainText('6 tracks');
});

test('a playlist of more than 30 tracks is refused on the link step', async ({ page, dialog }) => {
  await openPage(page);
  await page.keyboard.press('a');
  await dialog().getByRole('textbox', { name: 'Link' }).fill(tooLongLink);
  await page.keyboard.press('Enter');

  await expect(dialog().getByRole('alert')).toContainText('more than 30 tracks');
  await expect(dialog()).toBeVisible();
  await expect(page.getByRole('banner')).toContainText('6 tracks');
});

test('a watch link with a list in it opens the album review', async ({ page, dialog }) => {
  await openPage(page);
  await page.keyboard.press('a');
  await dialog().getByRole('textbox', { name: 'Link' }).fill('https://music.youtube.com/watch?v=abc&list=OLAK5uy_kindofblue');
  await page.keyboard.press('Enter');

  await expect(dialog().getByRole('textbox', { name: 'Album', exact: true })).toHaveValue('Kind of Blue');
  await expect(dialog().getByRole('checkbox')).toHaveCount(5);
});
