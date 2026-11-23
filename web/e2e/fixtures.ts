import { expect, test as base, type APIRequestContext, type JSHandle, type Locator, type Page } from '@playwright/test';
import type { LibraryResponse } from '../src/api/types.gen.ts';

interface Helpers {
  dialog: () => Locator;
}

interface Setup {
  seededLibrary: undefined;
}

export const test = base.extend<Helpers & Setup>({
  seededLibrary: [
    async ({ request }, use) => {
      const response = await request.post('/__e2e/reset', { headers: { 'X-MM': '1' } });
      expect(response.status()).toBe(204);
      await use(undefined);
    },
    { auto: true },
  ],
  dialog: async ({ page }, use) => {
    await use(() => page.getByRole('dialog'));
  },
});

export { expect };

export const headers = { 'X-MM': '1' };

export const pixel = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==';

export async function openPage(page: Page): Promise<void> {
  await page.goto('/');
  await expect(page.getByRole('banner')).toContainText('6 tracks');
  await expect(page.getByRole('treeitem').first()).toBeVisible();
}

export async function openAll(page: Page): Promise<void> {
  await openPage(page);
  await page.keyboard.press('Shift+ArrowRight');
  await page.keyboard.press('Shift+ArrowRight');
  await expect(row(page, 'So What')).toBeVisible();
}

export function items(page: Page): Locator {
  return page.getByRole('tree').getByRole('treeitem');
}

export function row(page: Page, label: string): Locator {
  const exactly = new RegExp(`^${label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}$`);
  return items(page).filter({ has: page.locator('.label bdi', { hasText: exactly }) });
}

export function pane(page: Page): Locator {
  return page.getByRole('region', { name: 'Details' });
}

export async function enterTrack(page: Page, label: string): Promise<void> {
  await row(page, label).last().click();
  await page.keyboard.press('Enter');
  await expect(pane(page).getByRole('heading', { level: 3 })).toHaveText(label);
}

export async function said(page: Page, text: string | RegExp): Promise<void> {
  await expect(page.getByRole('status')).toHaveText(text);
  await expect(page.getByTestId('app')).not.toHaveAttribute('aria-busy');
}

export async function boxOf(locator: Locator): Promise<{ x: number; y: number; width: number; height: number }> {
  const box = await locator.boundingBox();
  if (box === null) {
    throw new Error('the element has no box on the page');
  }
  return box;
}

export async function library(request: APIRequestContext): Promise<LibraryResponse> {
  return (await (await request.get('/api/library')).json()) as LibraryResponse;
}

export async function filesOf(page: Page, files: { name: string; type: string; base64: string }[]): Promise<JSHandle<DataTransfer>> {
  return page.evaluateHandle((list) => {
    const data = new DataTransfer();
    for (const f of list) {
      data.items.add(new File([Uint8Array.from(atob(f.base64), (c) => c.charCodeAt(0))], f.name, { type: f.type }));
    }
    return data;
  }, files);
}
