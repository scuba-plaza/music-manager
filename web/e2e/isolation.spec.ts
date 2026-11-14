import type { APIRequestContext } from '@playwright/test';
import type { LibraryResponse } from '../src/api/types.gen.ts';
import { expect, test } from './fixtures.ts';

test.describe.configure({ mode: 'serial' });

async function tracks(request: APIRequestContext) {
  const response = await request.get('/api/library?sort=title');
  const { view } = (await response.json()) as LibraryResponse;
  return view.rows.flatMap((row) => (row.kind === 'track' && row.trackId ? [{ id: row.trackId, label: row.label }] : []));
}

test('a test can change the library', async ({ request }) => {
  const soWhat = (await tracks(request)).find((track) => track.label === 'So What');
  if (!soWhat) {
    throw new Error('the seeded library has no So What');
  }
  const response = await request.patch(`/api/tracks/${soWhat.id}`, {
    headers: { 'X-MM': '1' },
    data: { field: 'title', text: 'So Different' },
  });
  expect(response.ok()).toBe(true);
  expect((await tracks(request)).map((track) => track.label)).toContain('So Different');
});

test('the next test starts from the seeded library again', async ({ request }) => {
  const labels = (await tracks(request)).map((track) => track.label);
  expect(labels).toContain('So What');
  expect(labels).not.toContain('So Different');
});
