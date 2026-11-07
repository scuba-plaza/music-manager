import type { Locator, Page } from '@playwright/test';
import { expect, test } from './fixtures.ts';

async function openPage(page: Page): Promise<void> {
  await page.goto('/');
  await expect(page.getByRole('banner')).toContainText('6 tracks');
  await expect(page.getByRole('treeitem').first()).toBeVisible();
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

async function openAll(page: Page): Promise<void> {
  await openPage(page);
  await page.keyboard.press('+');
  await page.keyboard.press('+');
  await expect(row(page, 'Rough Mix')).toBeVisible();
}

async function said(page: Page, text: string | RegExp): Promise<void> {
  await expect(page.getByRole('status')).toHaveText(text);
  await expect(page.getByTestId('app')).not.toHaveAttribute('aria-busy');
}

test('i on the unidentified track lists candidates, shows what would change, and applying identifies it', async ({ page, dialog }) => {
  await openAll(page);
  await row(page, 'Rough Mix').click();
  await expect(page.getByRole('banner')).toContainText('1 need work');

  await page.keyboard.press('i');

  await expect(dialog()).toHaveAccessibleName('Identify');
  const options = dialog().getByRole('option');
  await expect(options).toHaveCount(2);
  await expect(options.first()).toHaveAttribute('aria-selected', 'true');
  const diff = dialog().locator('.diff');
  await expect(diff).toContainText('Title');
  await expect(diff).toContainText('Rough Mix');
  await expect(diff).toContainText('So What');
  await expect(diff).toContainText('Kind of Blue');
  await page.keyboard.press('ArrowDown');
  await expect(options.nth(1)).toHaveAttribute('aria-selected', 'true');
  await expect(diff).toContainText('Kind of Blue (Legacy Edition)');
  await page.keyboard.press('ArrowUp');
  await expect(options.first()).toHaveAttribute('aria-selected', 'true');
  await expect(diff).not.toContainText('Legacy Edition');
  await page.keyboard.press('ArrowDown');

  await page.keyboard.press('Enter');

  await expect(dialog()).toHaveCount(0);
  await expect(page.getByRole('banner')).not.toContainText('need work');
  await expect(row(page, 'Kind of Blue (Legacy Edition)')).toBeVisible();
  await expect(row(page, 'Basement Tapes')).toHaveCount(0);
});

test('i with other words searches again, and esc leaves without changing anything', async ({ page, dialog }) => {
  await openAll(page);
  await row(page, 'Rough Mix').click();
  await page.keyboard.press('i');
  await expect(dialog().getByRole('option')).toHaveCount(2);

  await page.keyboard.press('/');
  const title = dialog().getByRole('textbox', { name: 'title' });
  await expect(title).toBeFocused();
  await expect(title).toHaveValue('Rough Mix');
  await title.fill('So What');
  await dialog().getByRole('textbox', { name: 'artist' }).fill('Miles Davis');
  await page.keyboard.press('Enter');

  await expect(dialog().getByRole('option')).toHaveCount(2);
  await expect(dialog()).toContainText('“So What – Miles Davis”');
  await page.keyboard.press('Escape');
  await expect(dialog()).toHaveCount(0);
  await expect(page.getByRole('banner')).toContainText('1 need work');
});

test('I identifies all with a progress bar', async ({ page, dialog }) => {
  await openAll(page);

  await page.keyboard.press('I');
  await expect(dialog()).toContainText('Identify 1 track on MusicBrainz?');
  await page.keyboard.press('y');

  await expect(dialog()).toHaveAccessibleName('Identify all');
  const bar = dialog().getByRole('progressbar');
  await expect(bar).toHaveAttribute('aria-valuemax', '1');
  await expect(bar).toHaveAttribute('aria-valuenow', '1');
  await expect(dialog()).toContainText('Done');
  await expect(dialog()).toContainText('Identified 1 · Left for you 0 · Failed 0');
  await expect(page.getByRole('banner')).not.toContainText('need work');
  await page.keyboard.press('Enter');
  await expect(dialog()).toHaveCount(0);
});

test('o adopts the file\'s old tags', async ({ page }) => {
  await openAll(page);
  await row(page, 'Rough Mix').click();

  await page.keyboard.press('o');

  await said(page, 'Used the old tags of 1 track');
  await expect(page.getByRole('banner')).not.toContainText('need work');
  await expect(row(page, 'Basement Tapes')).toBeVisible();
});

test('D lists the problems and a click moves the cursor to the track', async ({ page, dialog }) => {
  await openAll(page);

  await page.keyboard.press('D');

  await expect(dialog()).toHaveAccessibleName('Library check');
  await expect(dialog()).toContainText('0 errors, 1 warning');
  const problem = dialog().getByRole('option');
  await expect(problem).toHaveCount(1);
  await expect(problem).toContainText('Rough Mix');
  await expect(problem).toContainText('not identified yet');
  await problem.click();

  await expect(dialog()).toHaveCount(0);
  await expect(row(page, 'Rough Mix')).toHaveAttribute('aria-selected', 'true');
});
