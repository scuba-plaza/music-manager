import { expect, test as base, type Locator } from '@playwright/test';

interface Helpers {
  press: (key: string) => Promise<void>;
  tree: () => Locator;
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
  press: async ({ page }, use) => {
    await use((key) => page.keyboard.press(key));
  },
  tree: async ({ page }, use) => {
    await use(() => page.getByRole('tree'));
  },
  dialog: async ({ page }, use) => {
    await use(() => page.getByRole('dialog'));
  },
});

export { expect };
