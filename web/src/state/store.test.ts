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
  it('starts on the tree with nothing loaded, marked or sorted', async () => {
    vi.stubGlobal('localStorage', memoryStorage());

    const state = (await freshStore()).getState();

    expect(state).toMatchObject({
      info: null,
      lib: null,
      filter: '',
      cursor: null,
      focus: 'tree',
      field: 0,
      tab: 0,
      detail: null,
      edit: null,
      playing: null,
      status: { text: '', kind: '' },
      overlay: null,
      busy: false,
      sort: { key: '', reverse: false },
    });
    expect([...state.expanded]).toEqual([]);
    expect([...state.marks]).toEqual([]);
  });

  it('expanded survives a reload', async () => {
    const storage = memoryStorage();
    vi.stubGlobal('localStorage', storage);
    const first = await freshStore();

    first.setState({ expanded: new Set(['artist:a', 'album:b']) });
    const second = await freshStore();

    expect([...second.getState().expanded]).toEqual(['artist:a', 'album:b']);
    expect(storage.data.get('mm.expanded')).toBe('["artist:a","album:b"]');
  });

  it('the tab survives a reload', async () => {
    vi.stubGlobal('localStorage', memoryStorage());
    const first = await freshStore();

    first.setState({ tab: 2 });
    const second = await freshStore();

    expect(second.getState().tab).toBe(2);
  });

  it('writes nothing for a change to something else', async () => {
    const storage = memoryStorage();
    vi.stubGlobal('localStorage', storage);
    const store = await freshStore();

    store.setState({ filter: 'bach', busy: true });

    expect(storage.data.size).toBe(0);
  });

  it('a thrown localStorage does not break the store', async () => {
    vi.stubGlobal('localStorage', deniedStorage());

    const store = await freshStore();
    store.setState({ expanded: new Set(['artist:a']), tab: 3 });

    expect([...store.getState().expanded]).toEqual(['artist:a']);
    expect(store.getState().tab).toBe(3);
  });

  it('starts without trouble where there is no localStorage at all', async () => {
    vi.stubGlobal('localStorage', undefined);

    const store = await freshStore();
    store.setState({ tab: 1 });

    expect(store.getState().tab).toBe(1);
    expect([...store.getState().expanded]).toEqual([]);
  });

  it('falls back to the defaults for what was stored badly', async () => {
    vi.stubGlobal('localStorage', memoryStorage({ 'mm.expanded': '{"a":1}', 'mm.tab': 'two' }));

    const store = await freshStore();

    expect([...store.getState().expanded]).toEqual([]);
    expect(store.getState().tab).toBe(0);
  });

  it('keeps only the strings of a stored list', async () => {
    vi.stubGlobal('localStorage', memoryStorage({ 'mm.expanded': '["artist:a",7,null,"album:b"]' }));

    const store = await freshStore();

    expect([...store.getState().expanded]).toEqual(['artist:a', 'album:b']);
  });
});
