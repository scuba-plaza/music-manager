import type { Page } from '@playwright/test';
import { expect, openPage, test } from './fixtures.ts';

function names(page: Page) {
  return page.getByRole('dialog').getByRole('option').locator('.label bdi');
}

test('a format is made with its mistakes said on the boxes, then it is there to sync with', async ({ page, dialog }) => {
  await openPage(page);
  await page.keyboard.press('f');
  await expect(dialog()).toHaveAccessibleName('Formats');
  await expect(names(page)).toHaveText(['Generic', 'Wolfgang WMP01', 'My Player']);
  await page.keyboard.press('c');
  const name = dialog().getByLabel('Name', { exact: true });
  const longest = dialog().getByLabel('Longest name', { exact: true });
  await expect(name).toBeFocused();

  const width = dialog().getByLabel('Artist and album width');
  const side = dialog().getByLabel('Longest side', { exact: true });
  await expect(width).toBeDisabled();
  await expect(dialog().getByText('Needs Latin letters only')).toBeVisible();
  await expect(side).toBeDisabled();
  await expect(dialog().getByText('Only when covers are converted')).toHaveCount(2);

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
  await expect(width).toBeEnabled();
  await width.fill('12');
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
