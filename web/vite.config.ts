import { playwright } from '@vitest/browser-playwright';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';
import type { BrowserCommand } from 'vitest/node';
import { launchOptions } from './launch.ts';

const pixel = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==', 'base64');

const serveCovers: BrowserCommand = async ({ page }) => {
  await page.route('**/api/tracks/*/cover?*', (route) => route.fulfill({ contentType: 'image/png', body: pixel }));
};

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': { target: 'http://127.0.0.1:7878', changeOrigin: false },
    },
  },
  test: {
    passWithNoTests: true,
    projects: [
      {
        extends: true,
        test: {
          name: 'unit',
          environment: 'node',
          include: ['src/**/*.test.ts', 'eslint-rules/**/*.test.ts'],
        },
      },
      {
        extends: true,
        test: {
          name: 'browser',
          include: ['src/**/*.browser.test.tsx'],
          browser: {
            enabled: true,
            headless: true,
            provider: playwright({ launchOptions }),
            instances: [{ browser: 'chromium' }],
            commands: { serveCovers },
          },
        },
      },
    ],
  },
});
