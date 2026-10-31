import { defineConfig } from '@playwright/test';
import { launchOptions } from './launch.ts';

const baseURL = 'http://127.0.0.1:7979';

export default defineConfig({
  testDir: './e2e',
  workers: 1,
  use: { baseURL, launchOptions },
  webServer: {
    command:
      'pnpm run build && cd .. && go run ./cmd/e2e-server --addr 127.0.0.1:7979 --dist internal/web/dist --config web/e2e/fixtures/devices.toml',
    url: `${baseURL}/api/info`,
    reuseExistingServer: false,
    timeout: 180_000,
  },
  projects: [
    { name: 'desktop', use: { viewport: { width: 1200, height: 800 } } },
    { name: 'narrow', use: { viewport: { width: 500, height: 700 } } },
  ],
});
