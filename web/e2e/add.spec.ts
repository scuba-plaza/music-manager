import type { JSHandle, Locator, Page } from '@playwright/test';
import { expect, test } from './fixtures.ts';

const duplicateLink = 'https://www.youtube.com/watch?v=duplicate';
const frameSize = 417;

function silentMp3(frames: number): string {
  const frame = Buffer.alloc(frameSize);
  frame.set([0xff, 0xfb, 0x90, 0x00]);
  return Buffer.concat(Array.from({ length: frames }, () => frame)).toString('base64');
}

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

async function said(page: Page, text: string | RegExp): Promise<void> {
  await expect(page.getByRole('status')).toHaveText(text);
  await expect(page.getByTestId('app')).not.toHaveAttribute('aria-busy');
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

test('a adds from a link, pre-filled, and the path preview shows where it will be written', async ({ page, dialog }) => {
  await openPage(page);

  await page.keyboard.press('a');
  await expect(dialog()).toHaveAccessibleName('Add from YouTube');
  const link = dialog().getByRole('textbox', { name: 'Link' });
  await expect(link).toBeFocused();
  await link.fill('https://www.youtube.com/watch?v=abc');
  await page.keyboard.press('Enter');

  await expect(dialog().getByRole('textbox', { name: 'Title' })).toHaveValue('So What');
  await expect(dialog().getByRole('textbox', { name: 'Artist' })).toHaveValue('Miles Davis');
  await expect(dialog().getByRole('textbox', { name: 'Album', exact: true })).toHaveValue('Kind of Blue');
  await expect(dialog().getByRole('textbox', { name: 'Track', exact: true })).toHaveValue('1');
  await expect(dialog().getByRole('textbox', { name: 'Tracks on album' })).toHaveValue('5');
  await expect(dialog().getByRole('textbox', { name: 'Released' })).toHaveValue('1959-08-17');
  await expect(dialog()).toContainText('Matched on MusicBrainz');
  const path = dialog().locator('.pathline');
  await expect(path).toHaveText(/^Miles Davis\/Kind of Blue\/.*So What\.mp3$/);

  await dialog().getByRole('textbox', { name: 'Title' }).fill('So What (alternate take)');
  await expect(path).toHaveText(/So What \(alternate take\)\.mp3$/);
  await page.keyboard.press('Enter');

  await expect(dialog()).toHaveCount(0);
  await said(page, /^Added/);
  await expect(page.getByRole('banner')).toContainText('7 tracks');
});

test('F2 ignores the match and uses what the download said', async ({ page, dialog }) => {
  await openPage(page);
  await page.keyboard.press('a');
  await dialog().getByRole('textbox', { name: 'Link' }).fill('https://www.youtube.com/watch?v=abc');
  await page.keyboard.press('Enter');
  await expect(dialog().getByRole('textbox', { name: 'Title' })).toHaveValue('So What');

  await page.keyboard.press('F2');

  await expect(dialog()).toContainText('Ignoring the match');
  await expect(dialog().getByRole('textbox', { name: 'Title' })).toHaveValue('');
  await page.keyboard.press('Escape');
  await expect(dialog()).toHaveCount(0);
  await expect(page.getByRole('banner')).toContainText('6 tracks');
});

test('a duplicate is flagged', async ({ page, dialog }) => {
  await openPage(page);

  await page.keyboard.press('a');
  await dialog().getByRole('textbox', { name: 'Link' }).fill(duplicateLink);
  await page.keyboard.press('Enter');

  await expect(dialog()).toContainText('This audio is already in your library:');
  await expect(dialog()).toContainText('So What');
  await expect(dialog()).toContainText('Miles Davis');
  await page.keyboard.press('Enter');
  await expect(dialog()).toHaveCount(0);
  await expect(row(page, 'So What')).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByRole('banner')).toContainText('6 tracks');
});

test('dropping two mp3 files adds two unidentified tracks', async ({ page }) => {
  await openPage(page);
  const files = await filesOf(page, [
    { name: 'first.mp3', type: 'audio/mpeg', base64: silentMp3(30) },
    { name: 'second.mp3', type: 'audio/mpeg', base64: silentMp3(31) },
  ]);

  await page.dispatchEvent('[role="tree"]', 'dragover', { dataTransfer: files });
  await expect(page.locator('body')).toHaveClass(/dropping/);
  await page.dispatchEvent('[role="tree"]', 'drop', { dataTransfer: files });

  await said(page, 'Added 2 tracks · u undoes it');
  await expect(page.getByRole('banner')).toContainText('8 tracks');
  await expect(page.getByRole('banner')).toContainText('3 need work');
});

test('choosing MP3 files in the add dialog adds them', async ({ page, dialog }) => {
  await openPage(page);
  await page.keyboard.press('a');

  await dialog().getByLabel('MP3 files').setInputFiles([{ name: 'chosen.mp3', mimeType: 'audio/mpeg', buffer: Buffer.from(silentMp3(32), 'base64') }]);

  await expect(dialog()).toHaveCount(0);
  await said(page, 'Added 1 track · u undoes it');
  await expect(page.getByRole('banner')).toContainText('2 need work');
});
