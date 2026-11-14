import { existsSync } from 'node:fs';
import { mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import type { Locator, Page } from '@playwright/test';
import type { LibraryResponse } from '../src/api/types.gen.ts';
import { expect, test } from './fixtures.ts';

async function openPage(page: Page): Promise<void> {
  await page.goto('/');
  await expect(page.getByRole('banner')).toContainText('6 tracks');
  await expect(page.getByRole('treeitem').first()).toBeVisible();
}

async function inFolder(run: (dir: string) => Promise<void>): Promise<void> {
  const dir = await mkdtemp(join(tmpdir(), 'mm-sync-'));
  try {
    await run(dir);
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
}

function format(page: Page): Locator {
  return page.getByRole('dialog').getByRole('combobox', { name: 'Format' });
}

async function review(page: Page, dir: string, profile: string): Promise<void> {
  await page.keyboard.press('s');
  await expect(page.getByRole('dialog')).toHaveAccessibleName('Sync');
  await page.keyboard.press('End');
  await page.keyboard.press('Enter');
  await page.getByRole('dialog').getByRole('textbox', { name: 'Folder' }).fill(dir);
  await format(page).selectOption(profile);
  await page.getByRole('dialog').getByRole('textbox', { name: 'Folder' }).press('Enter');
  await expect(page.getByRole('dialog').getByRole('list', { name: 'What would change' })).toBeVisible();
}

test('s opens the targets and the library folder has no format row', async ({ page, dialog }) => {
  await openPage(page);

  await page.keyboard.press('s');

  await expect(dialog()).toHaveAccessibleName('Sync');
  const targets = dialog().getByRole('listbox', { name: 'Where to write' }).getByRole('option');
  await expect(targets.first()).toContainText('Library folder');
  await expect(targets.first()).toHaveAttribute('aria-selected', 'true');
  await expect(targets.last()).toContainText('Another folder…');
  await expect(dialog()).toContainText('Where should the music be written?');
  await expect(format(page)).toHaveCount(0);
  await expect(dialog()).toContainText('the library folder is always written the standard way');
  await page.keyboard.press('w');
  await expect(format(page)).toHaveCount(0);

  await page.keyboard.press('End');

  await expect(format(page)).toBeVisible();
  await expect(format(page)).toHaveValue('generic');
  await page.keyboard.press('Home');
  await expect(format(page)).toHaveCount(0);
  await page.keyboard.press('Escape');
  await expect(dialog()).toHaveCount(0);
});

test('a typed folder offers every profile, generic first, and w goes through them', async ({ page, dialog }) => {
  await openPage(page);
  await page.keyboard.press('s');
  await expect(dialog()).toHaveAccessibleName('Sync');
  await page.keyboard.press('End');

  await page.keyboard.press('w');
  await expect(format(page)).toHaveValue('wolfgang-wmp01');
  await page.keyboard.press('w');
  await expect(format(page)).toHaveValue('my-player');
  await page.keyboard.press('w');
  await expect(format(page)).toHaveValue('generic');

  await page.keyboard.press('Enter');

  await expect(dialog().getByRole('textbox', { name: 'Folder' })).toBeFocused();
  await expect(format(page)).toHaveValue('generic');
  await expect(format(page).getByRole('option')).toHaveText(['Generic', 'Wolfgang WMP01', 'My Player']);
  await expect(format(page).getByRole('option')).toHaveCount(3);
  await page.keyboard.press('Escape');
  await expect(dialog().getByRole('listbox', { name: 'Where to write' })).toBeVisible();
  await expect(format(page)).toHaveValue('generic');
});

test('choosing wolfgang-wmp01 shows its warnings in the review', async ({ page, dialog, request }) => {
  const { view } = (await (await request.get('/api/library')).json()) as LibraryResponse;
  const soWhat = view.rows.find((row) => row.kind === 'track' && row.label === 'So What');
  const patched = await request.patch(`/api/tracks/${soWhat?.trackId ?? ''}`, { headers: { 'X-MM': '1' }, data: { field: 'genre', text: 'Experimental jazz fusion' } });
  expect(patched.ok()).toBe(true);
  await openPage(page);

  await inFolder(async (dir) => {
    await review(page, dir, 'wolfgang-wmp01');

    await expect(dialog()).toContainText('Wolfgang WMP01');
    await expect(dialog().locator('.warns')).toHaveText('1 warning for the player');
    await expect(dialog()).toContainText('So What');
    await expect(dialog()).toContainText('the genre "Experimental jazz fusion" is 24 characters, the player shows 12');
    await page.keyboard.press('Escape');
    await expect(dialog()).toHaveCount(0);
  });
});

test('a profile from the user’s devices.toml appears and is the one that is planned', async ({ page, dialog }) => {
  await openPage(page);

  await inFolder(async (dir) => {
    await review(page, dir, 'my-player');

    await expect(dialog()).toContainText('My Player');
    await expect(dialog().locator('.warns')).toHaveCount(0);
    await page.keyboard.press('Escape');
  });
});

test('strays are removed only after typing delete', async ({ page, dialog }) => {
  await openPage(page);

  await inFolder(async (dir) => {
    await mkdir(join(dir, 'Old'));
    const stray = join(dir, 'Old', 'leftover.mp3');
    await writeFile(stray, 'not music');
    await review(page, dir, 'generic');
    await expect(dialog()).toContainText('Stray files are left alone. p also deletes the strays.');

    await page.keyboard.press('p');
    await expect(dialog()).toContainText('The files marked − will be deleted. p keeps them.');
    await page.keyboard.press('Enter');

    const word = dialog().getByRole('textbox', { name: 'Type delete to confirm' });
    await expect(word).toBeFocused();
    await expect(dialog()).toContainText('Type delete to remove 1 file');
    await word.fill('delet');
    await page.keyboard.press('Enter');
    await expect(word).toHaveValue('');
    expect(existsSync(stray)).toBe(true);

    await word.fill('delete');
    await page.keyboard.press('Enter');

    await expect(dialog()).toContainText('Done');
    expect(existsSync(stray)).toBe(false);
    await page.keyboard.press('Enter');
    await expect(dialog()).toHaveCount(0);
  });
});

test('the header says N to sync · press s and the chip opens the dialog', async ({ page, dialog }) => {
  await openPage(page);
  const chip = page.getByRole('banner').getByRole('button', { name: /to sync/ });
  await expect(chip).toHaveText(/^\d+ to sync · press s$/);

  await chip.click();

  await expect(dialog()).toHaveAccessibleName('Sync');
});
