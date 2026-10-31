import { page } from 'vitest/browser';
import { expect, test } from 'vitest';
import { render } from 'vitest-browser-react';
import { App } from './App';

test('renders the application shell', async () => {
  await render(<App />);
  await expect.element(page.getByRole('banner')).toHaveTextContent('mm');
});
