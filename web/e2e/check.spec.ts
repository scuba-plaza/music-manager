import { expect, openAll, row, test } from './fixtures.ts';

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
