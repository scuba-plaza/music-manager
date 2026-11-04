import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ChangeResponse, History, LibraryResponse, PeekView, TrackView } from '../api/types.gen';
import { installKeys } from '../keys/dispatch';
import {
  changeResponse,
  fieldView,
  infoView,
  jsonResponse,
  memoryStorage,
  requestsTo,
  sampleLibrary,
  stubServer,
  type StubRoute,
} from '../testing/fakes';
import { send } from '../api/client';
import { loadDetail, run } from './actions';
import { applyBulk, typeBulk, useBulk } from './bulk';
import { coverChosen, dropFiles, pastePicture, setCover } from './cover';
import { applyChange } from './edit';
import { useApp, type AppState } from './store';

vi.mock('./treeDom', () => ({
  pageSize: vi.fn(() => 3),
  centreCursor: vi.fn(),
  focusFilterBox: vi.fn(),
  leaveFilterBox: vi.fn(),
}));

const sep = '\u001f';
const keys = {
  playlists: 'playlists',
  lateNight: `playlist${sep}Late night`,
  lateBlue: `pltrack${sep}Late night${sep}blue`,
  needs: 'needs',
  rough: `track${sep}rough`,
  fairuz: `artist${sep}فيروز`,
  miles: `artist${sep}Miles Davis`,
  kindOfBlue: `album${sep}Miles Davis${sep}Kind of Blue`,
  soWhat: `track${sep}sowhat`,
  blue: `track${sep}blue`,
  nina: `artist${sep}Nina Simone`,
  singles: `singles${sep}Nina Simone`,
  good: `track${sep}good`,
};

const blankValues = {
  artist: '',
  artistLatin: '',
  album: '',
  albumLatin: '',
  of: '',
  released: '',
  genre: '',
  genreLatin: '',
  cutStart: '',
  cutEnd: '',
};

function library(history: History = {}, sortKey = ''): LibraryResponse {
  const base = sampleLibrary();
  return { ...base, history, view: { ...base.view, sort: { ...base.view.sort, key: sortKey } } };
}

function soWhat(): TrackView {
  return {
    id: 'sowhat',
    title: 'So What',
    identified: true,
    coverSha: 'abc123',
    tabs: [
      {
        id: 'details',
        name: 'Details',
        fields: [
          fieldView('title', { label: 'Title', text: 'So What', edit: 'So What' }),
          fieldView('artist', { label: 'Artist', text: 'Miles Davis', edit: 'Miles Davis' }),
          fieldView('genre', { label: 'Genre', text: 'Jazz', edit: 'Jazz' }),
          fieldView('cover', { label: 'Cover', text: '500×500 JPEG', empty: 'none' }),
        ],
      },
    ],
    path: 'Miles Davis/Kind of Blue/01 - So What.mp3',
    dates: '',
    about: '',
    reveal: [],
  };
}

function kindOfBlue(): PeekView {
  return {
    kind: 'album',
    title: 'Album',
    label: 'Kind of Blue',
    editable: true,
    info: [],
    fields: [fieldView('album', { label: 'Name', text: 'Kind of Blue', edit: 'Kind of Blue' }), fieldView('cover', { label: 'Cover', text: 'yes', empty: 'none' })],
    coverTrack: 'sowhat',
    coverSha: 'abc123',
    list: [],
  };
}

let fetchMock: ReturnType<typeof stubServer>;

const answers: StubRoute = ({ method, url }) => {
  const changes = [
    '/api/bulk',
    '/api/remove',
    '/api/move',
    '/api/renumber',
    '/api/undo',
    '/api/redo',
    '/api/paste',
    '/api/copy',
    '/api/playlists/takeout',
    '/api/playlists/delete',
  ];
  if ((method === 'POST' && changes.includes(url.pathname)) || ((method === 'PUT' || method === 'DELETE') && url.pathname === '/api/cover')) {
    return jsonResponse(changeResponse({ message: `answered ${url.pathname}` }));
  }
  if (method === 'POST' && url.pathname === '/api/bulk/preview') {
    return jsonResponse({ summary: 'Album → Blue', changed: 2 });
  }
  return undefined;
};

function serve(route: StubRoute = () => undefined, lib: LibraryResponse = library()): void {
  fetchMock = stubServer(infoView(), lib, { route: (request) => route(request) ?? answers(request) });
}

function show(cursor: string, extra: Partial<AppState> = {}): void {
  useApp.setState({ lib: library(), cursor, expanded: new Set([keys.playlists, keys.lateNight, keys.miles, keys.kindOfBlue, keys.nina, keys.singles]), ...extra });
}

function deferred(): { promise: Promise<Response>; resolve: (res: Response) => void } {
  let resolve: (res: Response) => void = () => undefined;
  const promise = new Promise<Response>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

function asked(): { head: string; body: string[]; yes: string } {
  const overlay = useApp.getState().overlay;
  if (overlay?.kind !== 'ask') {
    throw new Error(`no question was asked; the overlay is ${overlay?.kind ?? 'closed'}`);
  }
  return { head: overlay.head, body: overlay.body, yes: overlay.yes };
}

function sayYes(): void {
  const overlay = useApp.getState().overlay;
  if (overlay?.kind !== 'ask') {
    throw new Error('no question was asked');
  }
  useApp.setState({ overlay: null });
  overlay.onYes();
}

function status(): AppState['status'] {
  return useApp.getState().status;
}

const png = () => new File([new Uint8Array([0x89, 0x50, 0x4e, 0x47])], 'cover.png', { type: 'image/png' });

beforeEach(() => {
  vi.stubGlobal('localStorage', memoryStorage());
  useApp.setState(useApp.getInitialState(), true);
  useBulk.setState(useBulk.getInitialState(), true);
  serve();
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('the bulk form', () => {
  it('e with tracks marked opens one form for them, named by how many are marked', () => {
    show(keys.kindOfBlue, { marks: new Set(['sowhat', 'blue']) });

    run('bulkEdit');

    expect(useApp.getState().overlay).toEqual({ kind: 'bulk', ids: ['blue', 'sowhat'], head: 'Edit 2 marked tracks' });
    expect(useBulk.getState()).toEqual({ values: blankValues, summary: { text: 'Type what the tracks should have.', bad: false } });
  });

  it('with nothing marked, e goes into the fields of a track, an artist, an album or a playlist', () => {
    show(keys.soWhat);
    run('bulkEdit');
    expect(useApp.getState()).toMatchObject({ focus: 'inspector', overlay: null });

    show(keys.miles, { focus: 'tree', field: 3 });
    run('bulkEdit');
    expect(useApp.getState()).toMatchObject({ focus: 'inspector', field: 0, overlay: null });
  });

  it('e on Singles, which has no fields, edits its tracks in the form', () => {
    show(keys.singles);

    run('bulkEdit');

    expect(useApp.getState().overlay).toEqual({ kind: 'bulk', ids: ['good'], head: 'Edit 1 track' });
  });

  it('e on a playlist goes into its fields, even with tracks marked', () => {
    show(keys.lateNight, { marks: new Set(['sowhat']) });

    run('bulkEdit');

    expect(useApp.getState()).toMatchObject({ focus: 'inspector', overlay: null });
  });

  it('e on the Playlists group says to choose a playlist', () => {
    show(keys.playlists);

    run('bulkEdit');

    expect(status()).toEqual({ text: 'Select a playlist to edit it', kind: 'bad' });
    expect(useApp.getState().overlay).toBeNull();
  });

  it('the summary is asked for once the typing pauses for 150 ms', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    show(keys.kindOfBlue, { marks: new Set(['sowhat', 'blue']) });
    run('bulkEdit');

    typeBulk('album', 'B');
    await vi.advanceTimersByTimeAsync(100);
    typeBulk('album', 'Blue');
    await vi.advanceTimersByTimeAsync(149);
    expect(requestsTo(fetchMock, 'POST', '/api/bulk/preview')).toEqual([]);
    await vi.advanceTimersByTimeAsync(1);

    expect(requestsTo(fetchMock, 'POST', '/api/bulk/preview')).toEqual([{ ids: ['blue', 'sowhat'], values: { ...blankValues, album: 'Blue' } }]);
    expect(useBulk.getState().summary).toEqual({ text: 'Album → Blue', bad: false });
  });

  it('a stale bulk preview answer is dropped', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    const pending = [deferred(), deferred()];
    serve(({ method, url }) => (method === 'POST' && url.pathname === '/api/bulk/preview' ? pending[requestsTo(fetchMock, 'POST', '/api/bulk/preview').length - 1]?.promise : undefined));
    show(keys.kindOfBlue, { marks: new Set(['sowhat', 'blue']) });
    run('bulkEdit');

    typeBulk('album', 'Blu');
    await vi.advanceTimersByTimeAsync(150);
    typeBulk('album', 'Blue');
    await vi.advanceTimersByTimeAsync(150);
    expect(requestsTo(fetchMock, 'POST', '/api/bulk/preview')).toHaveLength(2);

    pending[1]?.resolve(jsonResponse({ summary: 'Album → Blue', changed: 2 }));
    await vi.waitFor(() => {
      expect(useBulk.getState().summary.text).toBe('Album → Blue');
    });
    pending[0]?.resolve(jsonResponse({ summary: 'Album → Blu', changed: 2 }));
    await vi.advanceTimersByTimeAsync(10);

    expect(useBulk.getState().summary).toEqual({ text: 'Album → Blue', bad: false });
  });

  it('values the server cannot use are said in the summary, as bad', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    serve(({ method, url }) =>
      method === 'POST' && url.pathname === '/api/bulk/preview'
        ? jsonResponse({ summary: 'Tracks on album → many', changed: 0, error: 'Tracks on the album must be a whole number' })
        : undefined,
    );
    show(keys.kindOfBlue, { marks: new Set(['sowhat']) });
    run('bulkEdit');

    typeBulk('of', 'many');
    await vi.advanceTimersByTimeAsync(150);

    expect(useBulk.getState().summary).toEqual({ text: 'Tracks on the album must be a whole number', bad: true });
  });

  it('an empty summary asks again for what the tracks should have', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    serve(({ method, url }) => (method === 'POST' && url.pathname === '/api/bulk/preview' ? jsonResponse({ summary: '', changed: 0 }) : undefined));
    show(keys.kindOfBlue, { marks: new Set(['sowhat']) });
    run('bulkEdit');
    useBulk.setState({ summary: { text: 'Album → Blue', bad: false } });

    typeBulk('album', '');
    await vi.advanceTimersByTimeAsync(150);

    expect(useBulk.getState().summary).toEqual({ text: 'Type what the tracks should have.', bad: false });
  });

  it('closing the form before the pause is over sends no preview request', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    show(keys.kindOfBlue, { marks: new Set(['sowhat']) });
    run('bulkEdit');

    typeBulk('album', 'Blue');
    useApp.setState({ overlay: null });
    await vi.advanceTimersByTimeAsync(300);

    expect(requestsTo(fetchMock, 'POST', '/api/bulk/preview')).toEqual([]);
  });

  it('an answer that arrives after the form was closed changes nothing', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    const pending = deferred();
    serve(({ method, url }) => (method === 'POST' && url.pathname === '/api/bulk/preview' ? pending.promise : undefined));
    show(keys.kindOfBlue, { marks: new Set(['sowhat']) });
    run('bulkEdit');
    typeBulk('album', 'Blue');
    await vi.advanceTimersByTimeAsync(150);
    expect(requestsTo(fetchMock, 'POST', '/api/bulk/preview')).toHaveLength(1);

    useApp.setState({ overlay: null });
    pending.resolve(jsonResponse({ summary: 'Album → Blue', changed: 1 }));
    await vi.advanceTimersByTimeAsync(10);

    expect(useBulk.getState().summary).toEqual({ text: 'Type what the tracks should have.', bad: false });
  });

  it('enter applies what was typed to the tracks as one change, and the form closes', async () => {
    show(keys.kindOfBlue, { marks: new Set(['sowhat', 'blue']) });
    run('bulkEdit');
    typeBulk('album', 'Blue');
    typeBulk('genreLatin', '-');

    await applyBulk();

    expect(requestsTo(fetchMock, 'POST', '/api/bulk')).toEqual([{ ids: ['blue', 'sowhat'], values: { ...blankValues, album: 'Blue', genreLatin: '-' } }]);
    expect(requestsTo(fetchMock, 'GET', '/api/library')).toHaveLength(1);
    expect(useApp.getState()).toMatchObject({ overlay: null, busy: false, status: { text: 'answered /api/bulk', kind: '' } });
  });

  it('a refusal is said in the form, which stays open', async () => {
    serve(({ method, url }) => (method === 'POST' && url.pathname === '/api/bulk' ? jsonResponse({ error: 'Nothing to change' }, 422) : undefined));
    show(keys.kindOfBlue, { marks: new Set(['sowhat']) });
    run('bulkEdit');

    await applyBulk();

    expect(useBulk.getState().summary).toEqual({ text: 'Nothing to change', bad: true });
    expect(useApp.getState()).toMatchObject({ overlay: { kind: 'bulk' }, busy: false });
  });
});

describe('one change at a time', () => {
  function keyboard() {
    let listener: ((e: KeyboardEvent) => void) | undefined;
    const doc = {
      addEventListener: (_: string, l: (e: KeyboardEvent) => void) => {
        listener = l;
      },
      removeEventListener: () => undefined,
    };
    const remove = installKeys(doc as unknown as Document, run);
    const press = (key: string, ctrlKey = false) => {
      const event = { key, ctrlKey, metaKey: false, altKey: false, shiftKey: false, isComposing: false, target: { tagName: 'BODY' }, preventDefault: () => undefined };
      listener?.(event as unknown as KeyboardEvent);
    };
    return { press, remove };
  }

  it('busy blocks keys until the change returns', async () => {
    const pending = deferred();
    serve(({ method, url }) => (method === 'POST' && url.pathname === '/api/undo' ? pending.promise : undefined));
    show(keys.soWhat, { lib: library({ undo: 'edit of So What' }) });
    const { press, remove } = keyboard();

    press('u');
    expect(useApp.getState().busy).toBe(true);
    press('j');
    press('u');
    expect(useApp.getState().cursor).toBe(keys.soWhat);
    expect(requestsTo(fetchMock, 'POST', '/api/undo')).toHaveLength(1);

    pending.resolve(jsonResponse(changeResponse({ message: 'Undid: edit of So What' })));
    await vi.waitFor(() => {
      expect(useApp.getState().busy).toBe(false);
    });
    press('j');

    expect(useApp.getState().cursor).toBe(keys.blue);
    remove();
  });

  it('details asked for before a change and answered after it do not undo what the change said', async () => {
    const late = deferred();
    const reload = deferred();
    let peeks = 0;
    serve(({ method, url }) => {
      if (method === 'GET' && url.pathname === '/api/peek') {
        peeks += 1;
        return peeks === 1 ? late.promise : undefined;
      }
      if (method === 'GET' && url.pathname === '/api/library') {
        return reload.promise;
      }
      return method === 'POST' && url.pathname === '/api/playlists/delete' ? jsonResponse(changeResponse({ message: 'Deleted the playlist Late night' })) : undefined;
    });
    show(keys.lateNight);
    const asked = loadDetail();

    const changed = applyChange(() => send<ChangeResponse>('POST', '/api/playlists/delete', { name: 'Late night' }));
    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'GET', '/api/library')).toHaveLength(1);
    });
    late.resolve(jsonResponse({ error: 'Not found' }, 404));
    await asked;
    reload.resolve(jsonResponse(library()));
    await changed;

    expect(status()).toEqual({ text: 'Deleted the playlist Late night', kind: '' });
    expect(useApp.getState().detail).toMatchObject({ label: 'Late night' });
  });

  it('details dropped by a change that does not read the library again are asked for again', async () => {
    serve(({ method, url }) => (method === 'POST' && url.pathname === '/api/paste' ? jsonResponse({ error: 'Nothing is copied yet' }, 422) : undefined));
    show(keys.soWhat);
    run('moveDown');

    await applyChange(() => send<ChangeResponse>('POST', '/api/paste', { ids: ['blue'] })).catch(() => undefined);

    await vi.waitFor(() => {
      expect(useApp.getState().detail).toMatchObject({ id: 'blue' });
    });
    expect(requestsTo(fetchMock, 'GET', '/api/tracks/blue')).toHaveLength(1);
  });

  it('a refused change asks for no details when none were on their way', async () => {
    serve(({ method, url }) => (method === 'POST' && url.pathname === '/api/paste' ? jsonResponse({ error: 'Nothing is copied yet' }, 422) : undefined));
    show(keys.soWhat);
    await loadDetail();

    await applyChange(() => send<ChangeResponse>('POST', '/api/paste', { ids: ['sowhat'] })).catch(() => undefined);
    await new Promise((resolve) => setTimeout(resolve, 20));

    expect(requestsTo(fetchMock, 'GET', '/api/tracks/sowhat')).toHaveLength(1);
  });

  it('a refused change says why in the status line and lets the keys go again', async () => {
    serve(({ method, url }) => (method === 'POST' && url.pathname === '/api/redo' ? jsonResponse({ error: 'another mm has the library' }, 422) : undefined));
    show(keys.soWhat, { lib: library({ redo: 'edit of So What' }) });

    run('redo');

    await vi.waitFor(() => {
      expect(status()).toEqual({ text: 'another mm has the library', kind: 'bad' });
    });
    expect(useApp.getState().busy).toBe(false);
  });
});

describe('removing', () => {
  const note = (many: boolean) => `Nothing is deleted from disk until you sync with prune, and u brings ${many ? 'them' : 'it'} back until then.`;

  it('d on a track asks first, and only then removes it', async () => {
    show(keys.soWhat);

    run('remove');

    expect(asked()).toEqual({ head: 'Remove “So What”?', body: [note(false)], yes: 'remove' });
    expect(fetchMock).not.toHaveBeenCalled();
    sayYes();
    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'POST', '/api/remove')).toEqual([{ ids: ['sowhat'] }]);
    });
    await vi.waitFor(() => {
      expect(status().text).toBe('answered /api/remove');
    });
  });

  it('the question names what goes: an artist and its tracks, the tracks of an album, everything in a group', () => {
    show(keys.miles);
    run('remove');
    expect(asked()).toEqual({ head: 'Remove “Miles Davis” and its 2 tracks?', body: ['• So What', '• Blue in Green', note(true)], yes: 'remove' });

    show(keys.kindOfBlue, { overlay: null });
    run('remove');
    expect(asked().head).toBe('Remove the 2 tracks of “Kind of Blue”?');

    show(keys.needs, { overlay: null });
    run('remove');
    expect(asked()).toEqual({ head: 'Remove everything in “Needs work” (1 track)?', body: [note(false)], yes: 'remove' });
  });

  it('with tracks marked, d removes the marked ones and lists the first five', async () => {
    show(keys.soWhat, { marks: new Set(['good', 'sowhat', 'giant', 'watani', 'rough', 'blue']) });

    run('remove');

    expect(asked()).toEqual({
      head: 'Remove 6 marked tracks?',
      body: ['• Blue in Green', '• Rough Mix', '• وطني', '• Giant Steps', '• So What', '…and 1 more', note(true)],
      yes: 'remove',
    });
    sayYes();
    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'POST', '/api/remove')).toEqual([{ ids: ['blue', 'rough', 'watani', 'giant', 'sowhat', 'good'] }]);
    });
  });

  it('d on a track of a playlist takes it out of the playlist, without asking', async () => {
    show(keys.lateBlue);

    run('remove');

    expect(useApp.getState().overlay).toBeNull();
    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'POST', '/api/playlists/takeout')).toEqual([{ name: 'Late night', ids: ['blue'] }]);
    });
    expect(requestsTo(fetchMock, 'POST', '/api/remove')).toEqual([]);
  });

  it('d in a playlist takes out the marked tracks of that playlist only', async () => {
    const lib = library();
    const at = lib.view.rows.findIndex((r) => r.key === keys.lateBlue);
    const lateSoWhat = { kind: 'track', depth: 2, key: `pltrack${sep}Late night${sep}sowhat`, parent: keys.lateNight, label: 'So What', trackId: 'sowhat' };
    lib.view.rows.splice(at + 1, 0, lateSoWhat);
    show(keys.lateBlue, { lib, marks: new Set(['sowhat', 'giant']) });

    run('remove');

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'POST', '/api/playlists/takeout')).toEqual([{ name: 'Late night', ids: ['sowhat'] }]);
    });
  });

  it('d in a playlist with no track of it marked takes out the track under the cursor', async () => {
    show(keys.lateBlue, { marks: new Set(['sowhat']) });

    run('remove');

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'POST', '/api/playlists/takeout')).toEqual([{ name: 'Late night', ids: ['blue'] }]);
    });
  });

  it('d on a playlist asks, and only then deletes it', async () => {
    show(keys.lateNight);

    run('remove');

    expect(asked()).toEqual({ head: 'Delete the playlist “Late night”?', body: ['Its 1 track stay in the library.', 'u brings the playlist back.'], yes: 'delete' });
    expect(fetchMock).not.toHaveBeenCalled();
    sayYes();
    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'POST', '/api/playlists/delete')).toEqual([{ name: 'Late night' }]);
    });
  });

  it('d on the Playlists group says what to choose', () => {
    show(keys.playlists);

    run('remove');

    expect(status()).toEqual({ text: 'Select a playlist, or one of its tracks, to take out of it', kind: 'bad' });
    expect(useApp.getState().overlay).toBeNull();
  });
});

describe('moving and numbering', () => {
  it('J and K move a track of an album down and up', async () => {
    show(keys.soWhat);

    run('moveTrackDown');
    await vi.waitFor(() => {
      expect(useApp.getState().busy).toBe(false);
      expect(requestsTo(fetchMock, 'POST', '/api/move')).toHaveLength(1);
    });
    run('moveTrackUp');

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'POST', '/api/move')).toEqual([
        { id: 'sowhat', dir: 1 },
        { id: 'sowhat', dir: -1 },
      ]);
    });
  });

  it('J in a playlist moves the track in the playlist', async () => {
    show(keys.lateBlue);

    run('moveTrackDown');

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'POST', '/api/move')).toEqual([{ id: 'blue', dir: 1, playlist: 'Late night' }]);
    });
  });

  it('only a track of an album or a playlist can be moved', () => {
    for (const key of [keys.good, keys.rough, keys.kindOfBlue]) {
      show(key);
      run('moveTrackDown');
      expect(status()).toEqual({ text: 'Only tracks of an album can be moved', kind: 'bad' });
    }
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('moving needs the tree view and the whole album or playlist in view', () => {
    show(keys.soWhat, { lib: library({}, 'title') });
    run('moveTrackDown');
    expect(status()).toEqual({ text: 'Moving a track needs the tree view · S, then Tree', kind: 'bad' });

    show(keys.soWhat, { filter: 'so' });
    run('moveTrackUp');
    expect(status()).toEqual({ text: 'Clear the filter first, so the whole album is in view', kind: 'bad' });

    show(keys.lateBlue, { filter: 'blue' });
    run('moveTrackUp');
    expect(status()).toEqual({ text: 'Clear the filter first, so the whole playlist is in view', kind: 'bad' });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('n numbers the album, from the album or one of its tracks', async () => {
    show(keys.blue);
    run('renumber');
    await vi.waitFor(() => {
      expect(useApp.getState().busy).toBe(false);
      expect(requestsTo(fetchMock, 'POST', '/api/renumber')).toHaveLength(1);
    });

    show(keys.kindOfBlue);
    run('renumber');

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'POST', '/api/renumber')).toEqual([{ key: keys.kindOfBlue }, { key: keys.kindOfBlue }]);
    });
  });

  it('n numbers only an album, in the tree view, with the whole album in view', () => {
    for (const key of [keys.singles, keys.good, keys.lateBlue, keys.needs, keys.miles]) {
      show(key);
      run('renumber');
      expect(status()).toEqual({ text: 'Select an album, or one of its tracks, to number it', kind: 'bad' });
    }
    show(keys.soWhat, { lib: library({}, 'title') });
    run('renumber');
    expect(status()).toEqual({ text: 'Numbering an album needs the tree view · S, then Tree', kind: 'bad' });

    show(keys.soWhat, { filter: 'so' });
    run('renumber');
    expect(status()).toEqual({ text: 'Clear the filter first, so the whole album is in view', kind: 'bad' });
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe('undo and redo', () => {
  it('u undoes and ctrl+r redoes the last change', async () => {
    const both = library({ undo: 'edit of So What', redo: 'paste of the cover' });
    serve(() => undefined, both);
    show(keys.soWhat, { lib: both });

    run('undo');
    await vi.waitFor(() => {
      expect(useApp.getState().busy).toBe(false);
      expect(requestsTo(fetchMock, 'POST', '/api/undo')).toHaveLength(1);
    });
    run('redo');

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'POST', '/api/redo')).toHaveLength(1);
    });
  });

  it('with nothing to undo or redo, u and ctrl+r say so and send nothing', () => {
    show(keys.soWhat);

    run('undo');
    expect(status()).toEqual({ text: 'Nothing to undo', kind: '' });
    run('redo');
    expect(status()).toEqual({ text: 'Nothing to redo', kind: '' });

    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('undoing puts the cursor on the track it brought back, and follows renamed rows', async () => {
    serve(({ method, url }) =>
      method === 'POST' && url.pathname === '/api/undo'
        ? jsonResponse(changeResponse({ message: 'Undid: removal of 1 track', reveal: 'blue', moves: { [`artist${sep}Miles`]: keys.miles } }))
        : undefined,
    );
    show(keys.needs, { lib: library({ undo: 'removal of 1 track' }), expanded: new Set([`artist${sep}Miles`]) });

    run('undo');

    await vi.waitFor(() => {
      expect(useApp.getState().cursor).toBe(keys.blue);
    });
    expect([...useApp.getState().expanded].sort()).toEqual([keys.kindOfBlue, keys.miles].sort());
  });
});

describe('copy and paste', () => {
  const copied = library({ clip: 'cover' });

  it('v pastes on the marked tracks, else on the track, album or artist under the cursor', async () => {
    const sent = async (cursor: string, marks: string[]) => {
      show(cursor, { lib: copied, marks: new Set(marks) });
      const before = requestsTo(fetchMock, 'POST', '/api/paste').length;
      run('paste');
      await vi.waitFor(() => {
        expect(useApp.getState().busy).toBe(false);
        expect(requestsTo(fetchMock, 'POST', '/api/paste')).toHaveLength(before + 1);
      });
      return requestsTo(fetchMock, 'POST', '/api/paste').at(-1);
    };

    expect(await sent(keys.needs, ['sowhat', 'blue'])).toEqual({ ids: ['blue', 'sowhat'] });
    expect(await sent(keys.soWhat, [])).toEqual({ ids: ['sowhat'] });
    expect(await sent(keys.kindOfBlue, [])).toEqual({ group: keys.kindOfBlue });
    expect(await sent(keys.miles, [])).toEqual({ group: keys.miles });
  });

  it('v on Singles, a group or a playlist says what to choose', () => {
    for (const key of [keys.singles, keys.needs, keys.lateNight]) {
      show(key, { lib: copied });
      run('paste');
      expect(status()).toEqual({ text: 'Select a track, an album or an artist to paste on', kind: 'bad' });
    }
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('v with nothing copied says how to copy', async () => {
    show(keys.soWhat);

    run('paste');

    await vi.waitFor(() => {
      expect(status()).toEqual({ text: 'Nothing is copied yet: c copies from a track · Ctrl+V pastes a picture', kind: 'bad' });
    });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('v with nothing copied pastes a picture on the clipboard as the cover', async () => {
    const picture = new Blob([new Uint8Array([0xff, 0xd8, 0xff])], { type: 'image/jpeg' });
    vi.stubGlobal('navigator', { clipboard: { read: () => Promise.resolve([{ types: ['text/plain', 'image/jpeg'], getType: () => Promise.resolve(picture) }]) } });
    show(keys.soWhat);

    run('paste');

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'PUT', '/api/cover?paste=1&name=clipboard.jpeg&ids=sowhat')).toHaveLength(1);
    });
    const sent = requestsTo(fetchMock, 'PUT', '/api/cover?paste=1&name=clipboard.jpeg&ids=sowhat')[0];
    expect(sent instanceof File ? [sent.type, sent.size] : sent).toEqual(['image/jpeg', 3]);
  });

  it('c in the fields of a track copies the field under the cursor', async () => {
    show(keys.soWhat, { focus: 'inspector', detail: soWhat(), field: 3 });

    run('copy');

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'POST', '/api/copy')).toEqual([{ id: 'sowhat', field: 'cover' }]);
    });
  });

  it('c in the fields of an album copies nothing', async () => {
    show(keys.kindOfBlue, { focus: 'inspector', detail: kindOfBlue() });

    run('copy');
    await new Promise((resolve) => setTimeout(resolve, 10));

    expect(fetchMock).not.toHaveBeenCalled();
    expect(useApp.getState().overlay).toBeNull();
  });
});

describe('covers', () => {
  const coverRequests = () => fetchMock.mock.calls.filter(([, init]) => init?.method === 'PUT');

  it('a picture that is not a JPEG or a PNG is refused and nothing is sent', async () => {
    show(keys.soWhat);

    await setCover(new File(['GIF89a'], 'cover.gif', { type: 'image/gif' }), { ids: ['sowhat'] });

    expect(status()).toEqual({ text: 'A cover must be a JPEG or PNG image', kind: 'bad' });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('a picture needs a track, an album or an artist to go to', async () => {
    show(keys.singles);

    await pastePicture(png());

    expect(status()).toEqual({ text: 'Select a track, an album or an artist for the cover', kind: 'bad' });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('a pasted picture goes to the marked tracks, sent as it is, and says where it came from', async () => {
    const file = png();
    show(keys.kindOfBlue, { marks: new Set(['sowhat', 'blue']) });

    await pastePicture(file);

    expect(requestsTo(fetchMock, 'PUT', '/api/cover?paste=1&name=cover.png&ids=blue%2Csowhat')).toEqual([file]);
    expect(status().text).toBe('answered /api/cover');
  });

  it('a picture dropped with nothing marked goes to the album or artist under the cursor', async () => {
    const file = png();
    show(keys.kindOfBlue);

    await dropFiles([new File(['notes'], 'notes.txt', { type: 'text/plain' }), file]);

    expect(requestsTo(fetchMock, 'PUT', `/api/cover?${new URLSearchParams({ group: keys.kindOfBlue }).toString()}`)).toEqual([file]);
  });

  it('a dropped file that is not a picture is refused with a message, and nothing is sent', async () => {
    show(keys.soWhat);

    await dropFiles([new File(['notes'], 'notes.txt', { type: 'text/plain' })]);
    expect(status()).toEqual({ text: 'Drop MP3 files to add them, or a JPEG or PNG picture to make it a cover', kind: 'bad' });

    await dropFiles([new File(['GIF89a'], 'cover.gif', { type: 'image/gif' })]);
    expect(status()).toEqual({ text: 'A cover must be a JPEG or PNG image', kind: 'bad' });

    expect(coverRequests()).toEqual([]);
  });

  it('the file chosen for the Cover field goes to what the pane shows, whatever is marked', async () => {
    const file = png();
    show(keys.soWhat, { focus: 'inspector', detail: soWhat(), field: 3, marks: new Set(['blue']) });

    coverChosen(file);

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'PUT', '/api/cover?ids=sowhat')).toEqual([file]);
    });
  });

  it('x on the Cover field of a track removes its cover at once', async () => {
    show(keys.soWhat, { focus: 'inspector', detail: soWhat(), field: 3 });

    run('removeCover');

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'DELETE', '/api/cover?ids=sowhat')).toHaveLength(1);
    });
  });

  it('x on the Cover field of an album removes the covers of its tracks', async () => {
    show(keys.kindOfBlue, { focus: 'inspector', detail: kindOfBlue(), field: 1 });

    run('removeCover');

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'DELETE', `/api/cover?${new URLSearchParams({ group: keys.kindOfBlue }).toString()}`)).toHaveLength(1);
    });
  });

  it('x says so when there is no cover to remove', () => {
    show(keys.soWhat, { focus: 'inspector', detail: { ...soWhat(), coverSha: '' }, field: 3 });

    run('removeCover');

    expect(status().text).toBe('There is no cover to remove');
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('x leaves any other field alone', () => {
    show(keys.soWhat, { focus: 'inspector', detail: soWhat(), field: 0 });

    run('removeCover');

    expect(fetchMock).not.toHaveBeenCalled();
  });
});
