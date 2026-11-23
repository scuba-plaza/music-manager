import type { Locator, Page } from '@playwright/test';
import { enterTrack, expect, openAll, pane, row, test } from './fixtures.ts';

async function intoFields(page: Page, label: string): Promise<void> {
  await openAll(page);
  await enterTrack(page, label);
}

test('editing the title saves it and the tree row follows', async ({ page }) => {
  await intoFields(page, 'So What');

  await page.keyboard.press('Enter');
  const title = pane(page).getByRole('textbox', { name: 'Title' });
  await expect(title).toHaveValue('So What');
  await title.fill('So What (take 2)');
  await page.keyboard.press('Enter');

  await expect(title).toHaveCount(0);
  await expect(pane(page).getByRole('heading', { level: 3 })).toHaveText('So What (take 2)');
  await expect(pane(page).getByTestId('value-title')).toHaveText('So What (take 2)');
  await page.keyboard.press('Escape');
  await expect(row(page, 'So What (take 2)')).toHaveAttribute('aria-selected', 'true');
  await expect(row(page, 'So What')).toHaveCount(0);
});

test('a refused title stays open with the reason under it', async ({ page }) => {
  await intoFields(page, 'So What');

  await page.keyboard.press('Enter');
  await pane(page).getByRole('textbox', { name: 'Title' }).fill('');
  await page.keyboard.press('Enter');

  await expect(pane(page).getByRole('alert')).not.toBeEmpty();
  await expect(pane(page).getByRole('textbox', { name: 'Title' })).toBeVisible();
  await expect(pane(page).getByRole('textbox', { name: 'Title' })).toHaveAttribute('aria-invalid', 'true');
  await page.keyboard.press('Escape');
  await expect(pane(page).getByTestId('value-title')).toHaveText('So What');
});

test('a release date that is no date is said under the box, with the shapes a date may have', async ({ page }) => {
  await intoFields(page, 'So What');
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  const released = pane(page).getByRole('textbox', { name: 'Released' });

  await released.fill('soon');
  await page.keyboard.press('Enter');

  await expect(pane(page).getByRole('alert')).toContainText('1999-05-17');
  await expect(released).toHaveAttribute('aria-invalid', 'true');
  await released.fill('1959');
  await expect(pane(page).getByRole('alert')).toHaveCount(0);
  await page.keyboard.press('Escape');
});

test('a total below the track number is said under the total, and the track number is left alone', async ({ page }) => {
  await intoFields(page, 'So What');
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  const number = pane(page).getByRole('textbox', { name: 'Track', exact: true });
  const total = pane(page).getByRole('textbox', { name: 'Tracks on album' });

  await number.fill('5');
  await total.fill('3');
  await page.keyboard.press('Enter');

  await expect(pane(page).getByRole('alert')).toContainText('cannot be fewer than the track number');
  await expect(total).toHaveAttribute('aria-invalid', 'true');
  await expect(number).not.toHaveAttribute('aria-invalid', 'true');
  await total.fill('9');
  await expect(pane(page).getByRole('alert')).toHaveCount(0);
  await page.keyboard.press('Escape');
});

test('the Latin tab shows a hand-set form as normal text and a worked-out one as dim', async ({ page }) => {
  await intoFields(page, 'وطني');

  await page.keyboard.press('3');

  const handSet = pane(page).getByTestId('value-latinTitle');
  const workedOut = pane(page).getByTestId('value-latinArtist');
  await expect(handSet).toHaveText('Watani');
  await expect(workedOut).toHaveText('Firuz');
  await expect(handSet).not.toHaveClass(/\bdim\b/);
  await expect(workedOut).toHaveClass(/\bdim\b/);
  const colour = (target: Locator) => target.evaluate((el) => getComputedStyle(el).color);
  expect(await colour(handSet)).not.toBe(await colour(workedOut));
});

test('the cover is shown for a track that has one', async ({ page }) => {
  await intoFields(page, 'So What');

  const cover = pane(page).getByRole('img', { name: 'Cover' });
  await expect(cover).toBeVisible();
  await expect.poll(() => cover.evaluate((img: HTMLImageElement) => img.naturalWidth)).toBeGreaterThan(0);

  await page.keyboard.press('Escape');
  await enterTrack(page, 'Giant Steps');
  await expect(cover).toHaveCount(0);
  await expect(pane(page).getByTestId('value-cover')).toHaveText('none');
});

test('the device tab previews the track with the profile chosen', async ({ page }) => {
  await intoFields(page, 'وطني');

  await pane(page).locator('[data-tab="device"]').click();
  const profile = pane(page).getByRole('combobox', { name: 'Format' });
  await expect(profile).toHaveValue('wolfgang-wmp01');
  await expect(pane(page).getByText('01 - Watani.mp3')).toBeVisible();

  const answer = page.waitForResponse((res) => res.url().includes('profile=my-player'));
  await profile.selectOption('my-player');
  expect((await answer).status()).toBe(200);

  await expect(profile).toHaveValue('my-player');
  await expect(pane(page).getByText('01 - Watani.mp3')).toBeVisible();
});
