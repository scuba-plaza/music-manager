import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-react';
import { App } from './App';
import { follow } from './state/jobs';
import { FakeEventSource, stubServer } from './testing/fakes';

beforeEach(() => {
  FakeEventSource.instances = [];
  stubServer();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

test('renders the application shell', async () => {
  await render(<App />);
  await expect.element(page.getByRole('banner')).toHaveTextContent('mm');
});

test('a job event from the server reaches the job being followed', async () => {
  await render(<App />);
  const shown: string[] = [];
  const ended = follow({ id: 'j9', kind: 'identify', state: 'running' }, (v) => shown.push(v.state));

  const source = FakeEventSource.instances[0];
  source?.emit('job', JSON.stringify({ id: 'j9', kind: 'identify', state: 'done' }));

  await expect(ended).resolves.toMatchObject({ id: 'j9', state: 'done' });
  expect(shown).toEqual(['running', 'done']);
});
