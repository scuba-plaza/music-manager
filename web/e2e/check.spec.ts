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
  await page.keyboard.press('Shift+ArrowRight');
  await page.keyboard.press('Shift+ArrowRight');
  await expect(row(page, 'Rough Mix')).toBeVisible();
}

test('D lists the problems and a click moves the cursor to the track', async ({ page, dialog }) => {
  await openAll(page);

  await page.keyboard.press('Shift+v');

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
