import type { LibraryResponse, ProfileView } from '../src/api/types.gen.ts';
import { expect, test } from './fixtures.ts';

test('the seeded server answers /api/library with four artists, two playlists and one unidentified track', async ({ request }) => {
  const response = await request.get('/api/library');
  expect(response.ok()).toBe(true);
  const { view } = (await response.json()) as LibraryResponse;

  const artists = view.rows.filter((row) => row.kind === 'artist').map((row) => row.label);
  expect(artists.toSorted()).toEqual(['John Coltrane', 'Miles Davis', 'Nina Simone', 'فيروز'].toSorted());

  expect(view.playlists.map((playlist) => [playlist.name, playlist.count])).toEqual([
    ['Late night', 1],
    ['A rather long playlist name that goes on and on and on for a while', 2],
  ]);

  expect(view.tracks).toBe(6);
  expect(view.needWork).toBe(1);
  expect(view.rows.find((row) => row.key === 'needs')).toMatchObject({ kind: 'group', label: 'Needs work', count: 1 });
});

test('/api/profiles lists generic, wolfgang-wmp01 and my-player', async ({ request }) => {
  const response = await request.get('/api/profiles');
  expect(response.ok()).toBe(true);
  const profiles = (await response.json()) as ProfileView[];
  expect(profiles.map((profile) => profile.name)).toEqual(['generic', 'wolfgang-wmp01', 'my-player']);
});
