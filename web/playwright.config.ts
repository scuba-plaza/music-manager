import { defineConfig } from '@playwright/test';
import { launchOptions } from './launch.ts';

export default defineConfig({
  testDir: './e2e',
  use: { launchOptions },
  projects: [
    { name: 'desktop', use: { viewport: { width: 1200, height: 800 } } },
    { name: 'narrow', use: { viewport: { width: 500, height: 700 } } },
  ],
});
