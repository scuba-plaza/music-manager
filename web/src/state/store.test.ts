import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { deniedStorage, memoryStorage } from '../testing/fakes';

async function freshStore() {
  vi.resetModules();
  return (await import('./store')).useApp;
}

beforeEach(() => {
  vi.resetModules();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('the store', () => {
  it('expanded survives a reload', async () => {
    const storage = memoryStorage();
    vi.stubGlobal('localStorage', storage);
    const first = await freshStore();

    first.setState({ expanded: new Set(['artist:a', 'album:b']) });
    const second = await freshStore();

    expect([...second.getState().expanded]).toEqual(['artist:a', 'album:b']);
    expect(storage.data.get('mm.expanded')).toBe('["artist:a","album:b"]');
  });

  it('writes nothing for a change to something else', async () => {
    const storage = memoryStorage();
    vi.stubGlobal('localStorage', storage);
    const store = await freshStore();

    store.setState({ filter: 'bach', busy: true });

    expect(storage.data.size).toBe(0);
  });

  it.each([
    ['throws', deniedStorage()],
    ['is not there at all', undefined],
  ])('works where localStorage %s', async (_, storage) => {
    vi.stubGlobal('localStorage', storage);

    const store = await freshStore();
    store.setState({ expanded: new Set(['artist:a']) });

    expect([...store.getState().expanded]).toEqual(['artist:a']);
  });

  it.each([
    ['is no list', '{"a":1}', []],
    ['is cut off', '["a', []],
    ['holds more than strings', '["artist:a",7,null,"album:b"]', ['artist:a', 'album:b']],
  ])('keeps only the strings of what was stored when it %s', async (_, stored, expanded) => {
    vi.stubGlobal('localStorage', memoryStorage({ 'mm.expanded': stored }));

    const store = await freshStore();

    expect([...store.getState().expanded]).toEqual(expanded);
  });
});
