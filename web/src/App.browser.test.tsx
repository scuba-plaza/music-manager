import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-react';
import { App } from './App';
import { stubServer } from './testing/fakes';

beforeEach(() => {
  stubServer();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

test('renders the application shell', async () => {
  await render(<App />);
  await expect.element(page.getByRole('banner')).toHaveTextContent('mm');
});
