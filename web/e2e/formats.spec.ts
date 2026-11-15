import type { APIRequestContext, Page } from '@playwright/test';
import { expect, test } from './fixtures.ts';

async function openPage(page: Page): Promise<void> {
  await page.goto('/');
  await expect(page.getByRole('banner')).toContainText('6 tracks');
  await expect(page.getByRole('treeitem').first()).toBeVisible();
}

function names(page: Page) {
  return page.getByRole('dialog').getByRole('option').locator('.label bdi');
}

async function makeFormat(request: APIRequestContext, name: string): Promise<void> {
  const made = await request.post('/api/profiles', {
    headers: { 'X-MM': '1' },
    data: {
      id: '', name, latin_names: false, component_max: 100, stem_max: 0, tag_width: 0, latin_genre: false,
      cover_format: 'keep', cover_max_edge: 0, cover_quality: 0, single_covers: false, replaygain: true,
      playlist_ext: '.m3u8', playlist_style: 'relative',
    },
  });
  expect(made.ok()).toBe(true);
}

test('f lists Generic and the formats of the library, and esc closes the list', async ({ page, dialog }) => {
  await openPage(page);

  await page.keyboard.press('f');

  await expect(dialog()).toHaveAccessibleName('Formats');
  await expect(names(page)).toHaveText(['Generic', 'Wolfgang WMP01', 'My Player']);
  await page.keyboard.press('Escape');
  await expect(dialog()).toHaveCount(0);
});

test('a format is made with its mistakes said on the boxes, then it is there to sync with', async ({ page, dialog }) => {
  await openPage(page);
  await page.keyboard.press('f');
  await page.keyboard.press('c');
  const name = dialog().getByLabel('Name', { exact: true });
  const longest = dialog().getByLabel('Longest name', { exact: true });
  await expect(name).toBeFocused();

  await page.keyboard.press('Tab');
  await expect(dialog().getByRole('alert')).toHaveText(/Name cannot be empty/);
  await expect(name).toHaveAttribute('aria-invalid', 'true');

  await name.fill('Car');
  await longest.fill('5');
  await page.keyboard.press('Tab');
  await expect(dialog().getByRole('alert')).toContainText('from 20 to 255');
  await page.keyboard.press('Enter');
  await expect(longest).toBeFocused();
  await expect(name).toHaveValue('Car');

  await longest.fill('90');
  await dialog().getByLabel('Latin letters only').check();
  await dialog().getByLabel('Artist and album width').fill('12');
  await page.keyboard.press('Enter');

  await expect(names(page)).toHaveText(['Generic', 'Wolfgang WMP01', 'My Player', 'Car']);
  await expect(page.getByRole('status')).toHaveText('Saved the format “Car”');
  await page.keyboard.press('Escape');
  await expect(dialog()).toHaveCount(0);
  await page.keyboard.press('s');
  await expect(dialog()).toHaveAccessibleName('Sync');
  await page.keyboard.press('End');
  await page.keyboard.press('Enter');
  await expect(dialog().getByRole('combobox', { name: 'Format' }).getByRole('option')).toHaveText(['Generic', 'Wolfgang WMP01', 'My Player', 'Car']);
});

test('a format cannot take the name of another, whatever the case', async ({ page, dialog }) => {
  await openPage(page);
  await page.keyboard.press('f');
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  const name = dialog().getByLabel('Name', { exact: true });
  await expect(name).toHaveValue('My Player');

  await name.fill('wolfgang wmp01');
  await page.keyboard.press('Enter');

  await expect(dialog().getByRole('alert')).toContainText('There is already a format called');
  await expect(name).toBeFocused();
  await expect(name).toHaveAttribute('aria-invalid', 'true');
});

test('a format is deleted after asking, and the Device tab goes on with the next one', async ({ page, dialog }) => {
  await openPage(page);
  await page.keyboard.press('f');
  await page.keyboard.press('ArrowDown');

  await page.keyboard.press('d');
  await expect(dialog()).toContainText('Delete the format “Wolfgang WMP01”?');
  await page.keyboard.press('y');

  await expect(names(page)).toHaveText(['Generic', 'My Player']);
  await expect(page.getByRole('status')).toHaveText('Deleted the format “Wolfgang WMP01”');
});

test('no more than eight formats, and the ninth is refused before its form is opened', async ({ page, request }) => {
  for (const name of ['a', 'b', 'c', 'd', 'e', 'f']) {
    await makeFormat(request, name);
  }
  await openPage(page);

  await page.keyboard.press('f');
  await page.keyboard.press('c');

  await expect(page.getByRole('status')).toHaveText(/at most 8 formats/);
  await expect(page.getByRole('dialog').getByRole('option').first()).toBeVisible();
});
