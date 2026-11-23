import { enterTrack, expect, headers, library, openAll, pane, row, said, test } from './fixtures.ts';

test('→ nudges a cut by a tenth and ← takes it back, Shift+→ and Shift+← do the same by a second, and d clears it', async ({ page }) => {
  await openAll(page);
  await enterTrack(page, 'So What');
  await page.keyboard.press('2');
  await pane(page).getByTestId('value-cutStart').click();
  await page.keyboard.press('Escape');
  const value = pane(page).getByTestId('value-cutStart');
  const blank = (await value.textContent()) ?? '';

  await page.keyboard.press('ArrowRight');
  await expect(value).not.toHaveText(blank);
  await expect(value).toContainText('1');

  await page.keyboard.press('ArrowLeft');
  await expect(value).toHaveText(blank);

  await page.keyboard.press('Shift+ArrowRight');
  await expect(value).not.toHaveText(blank);
  const second = (await value.textContent()) ?? '';

  await page.keyboard.press('Shift+ArrowLeft');
  await expect(value).not.toHaveText(second);
  await expect(value).toHaveText(blank);

  await page.keyboard.press('ArrowRight');
  await expect(value).not.toHaveText(blank);

  await page.keyboard.press('d');
  await expect(value).toHaveText(blank);
});

test('d on the Cover field asks first, and y removes the cover of the track', async ({ page }) => {
  await openAll(page);
  await enterTrack(page, 'So What');
  await pane(page).getByTestId('value-cover').click();
  await expect(pane(page).getByRole('img', { name: 'Cover' })).toBeVisible();

  await page.keyboard.press('d');

  const dialog = page.getByRole('dialog');
  await expect(dialog).toContainText('Remove the cover of “So What”?');
  await expect(pane(page).getByRole('img', { name: 'Cover' })).toBeVisible();
  await page.keyboard.press('y');

  await said(page, 'Saved');
  await expect(pane(page).getByRole('img', { name: 'Cover' })).toHaveCount(0);
});

test('what another program changes shows on the page within a few seconds', async ({ page, request }) => {
  await openAll(page);
  const giant = (await library(request)).view.rows.find((r) => r.kind === 'track' && r.label === 'Giant Steps');

  const patched = await request.patch(`/api/tracks/${giant?.trackId ?? ''}`, { headers, data: { field: 'title', text: 'Giant Strides' } });
  expect(patched.ok()).toBe(true);

  await expect(row(page, 'Giant Strides').first()).toBeVisible({ timeout: 8000 });
});

test('click selects, double-click opens, click on a field then again edits, the twist arrow opens at once', async ({ page }) => {
  await openAll(page);
  await page.keyboard.press('Shift+ArrowLeft');
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

test('the fields of another track open on the Details tab, not on the tab that was left', async ({ page }) => {
  await openAll(page);
  await enterTrack(page, 'So What');
  await page.keyboard.press('2');
  await expect(pane(page).getByRole('tab', { selected: true })).toHaveText('2 Audio');

  await page.keyboard.press('Escape');
  await row(page, 'Giant Steps').last().click();
  await page.keyboard.press('Enter');

  await expect(pane(page).getByRole('tab', { selected: true })).toHaveText('1 Details');
});
