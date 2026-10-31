import type { Locator, Page } from '@playwright/test';
import type { InfoView, LibraryResponse } from '../src/api/types.gen.ts';
import { expect, test } from './fixtures.ts';

async function boxOf(locator: Locator): Promise<{ x: number; y: number; width: number; height: number }> {
  const box = await locator.boundingBox();
  if (box === null) {
    throw new Error('the element has no box on the page');
  }
  return box;
}

async function openPage(page: Page, width: number, height = 700): Promise<void> {
  await page.setViewportSize({ width, height });
  await page.goto('/');
  await expect(page.getByRole('banner')).toContainText('6 tracks');
}

async function dragDivider(page: Page, to: number): Promise<void> {
  const divider = await boxOf(page.getByRole('separator'));
  const y = divider.y + divider.height / 2;
  await page.mouse.move(divider.x + divider.width / 2, y);
  await page.mouse.down();
  await page.mouse.move(to, y, { steps: 8 });
  await page.mouse.up();
}

test('the header shows the library folder, the tracks and what waits to sync', async ({ page, request }) => {
  const info = (await (await request.get('/api/info')).json()) as InfoView;
  const { unsynced } = (await (await request.get('/api/library')).json()) as LibraryResponse;
  expect(unsynced.n).toBeGreaterThan(0);

  await openPage(page, 1200, 800);

  const header = page.getByRole('banner');
  await expect(header).toContainText(info.root);
  await expect(header).toContainText('6 tracks · 1 need work');
  await expect(header.getByRole('button', { name: /to sync/ })).toHaveText(`${String(unsynced.n)} to sync · press s`);
  await expect(page).toHaveTitle(`mm · ${info.root}`);
});

test('below 800px one pane shows at a time: → into the fields shows the right pane and ← returns to the tree', async ({ page, press }) => {
  await openPage(page, 500);
  const tree = page.getByTestId('tree-pane');
  const fields = page.getByTestId('inspector-pane');
  await expect(tree).toBeVisible();
  await expect(fields).toBeHidden();
  expect((await boxOf(tree)).width).toBeGreaterThan(400);

  await press('ArrowRight');

  await expect(fields).toBeVisible();
  await expect(tree).toBeHidden();
  expect((await boxOf(fields)).width).toBeGreaterThan(400);

  await press('ArrowLeft');

  await expect(tree).toBeVisible();
  await expect(fields).toBeHidden();
});

test('both panes show from 800px and one pane below it', async ({ page }) => {
  await openPage(page, 800);
  await expect(page.getByTestId('tree-pane')).toBeVisible();
  await expect(page.getByTestId('inspector-pane')).toBeVisible();
  await expect(page.getByRole('separator')).toBeVisible();

  await page.setViewportSize({ width: 799, height: 700 });

  await expect(page.getByTestId('tree-pane')).toBeVisible();
  await expect(page.getByTestId('inspector-pane')).toBeHidden();
  await expect(page.getByRole('separator')).toBeHidden();
});

test('dragging the divider changes the left width and survives a reload', async ({ page }) => {
  await openPage(page, 1200, 800);
  const library = page.getByRole('region', { name: 'Library' });
  const before = (await boxOf(library)).width;
  const divider = await boxOf(page.getByRole('separator'));

  await dragDivider(page, divider.x + 150);

  const after = (await boxOf(library)).width;
  expect(after).toBeGreaterThan(before + 100);

  await page.reload();
  await expect(library).toBeVisible();

  expect((await boxOf(library)).width).toBeCloseTo(after, 0);
});

test('the divider leaves the left pane 220px and the right pane 300px', async ({ page }) => {
  await openPage(page, 1200, 800);
  const library = page.getByRole('region', { name: 'Library' });
  const details = page.getByRole('region', { name: 'Details' });

  await dragDivider(page, 2);

  expect((await boxOf(library)).width).toBeCloseTo(220, 0);

  await dragDivider(page, 1198);

  expect((await boxOf(details)).width).toBeCloseTo(300, 0);
});

test('a stored width that leaves a pane no room is brought back, and one that is no number is ignored', async ({ page }) => {
  await openPage(page, 1200, 800);
  const library = page.getByRole('region', { name: 'Library' });
  const details = page.getByRole('region', { name: 'Details' });
  const standard = (await boxOf(library)).width;

  await page.evaluate(() => {
    localStorage.setItem('mm.left', '5000');
  });
  await page.reload();
  await expect(details).toBeVisible();
  expect((await boxOf(details)).width).toBeGreaterThanOrEqual(300);

  await page.evaluate(() => {
    localStorage.setItem('mm.left', '5');
  });
  await page.reload();
  await expect(library).toBeVisible();
  expect((await boxOf(library)).width).toBeGreaterThanOrEqual(220);

  await page.evaluate(() => {
    localStorage.setItem('mm.left', 'wide');
  });
  await page.reload();
  await expect(library).toBeVisible();
  expect((await boxOf(library)).width).toBeCloseTo(standard, 0);
});

test('the theme starts dark and the light one survives a reload', async ({ page }) => {
  await openPage(page, 1200, 800);
  const root = page.locator('html');
  await expect(root).toHaveAttribute('data-theme', 'dark');

  await page.getByRole('button', { name: 'Switch theme' }).click();
  await expect(root).toHaveAttribute('data-theme', 'light');

  await page.reload();

  await expect(root).toHaveAttribute('data-theme', 'light');
});

test('no horizontal scrollbar at 500px', async ({ page, press }) => {
  await openPage(page, 500);
  const overflow = () =>
    page.evaluate(() => ({
      page: document.documentElement.scrollWidth - document.documentElement.clientWidth,
      body: document.body.scrollWidth - document.body.clientWidth,
    }));

  expect((await overflow()).page).toBeLessThanOrEqual(0);
  expect((await overflow()).body).toBeLessThanOrEqual(0);

  await press('ArrowRight');
  await expect(page.getByTestId('inspector-pane')).toBeVisible();

  expect((await overflow()).page).toBeLessThanOrEqual(0);
  expect((await overflow()).body).toBeLessThanOrEqual(0);
  const header = await boxOf(page.getByRole('banner'));
  expect(header.x + header.width).toBeLessThanOrEqual(500);
});

test('a long library path is cut short in the header and the page does not scroll sideways', async ({ page }) => {
  await openPage(page, 500);
  await page.locator('.root bdi').evaluate((path) => {
    path.textContent = '/media/'.concat('a-very-long-folder-name/'.repeat(20));
  });

  const root = await boxOf(page.locator('.root'));
  expect(root.x + root.width).toBeLessThanOrEqual(500);
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(overflow).toBeLessThanOrEqual(0);
});
