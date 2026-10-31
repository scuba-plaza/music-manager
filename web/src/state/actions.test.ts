import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '../api/client';
import type { LibraryResponse } from '../api/types.gen';
import {
  infoView,
  jsonResponse,
  libraryResponse,
  memoryStorage,
  narrowRows,
  peekViewFor,
  requestOf,
  sampleLibrary,
  stubServer,
} from '../testing/fakes';
import {
  composeFilter,
  fail,
  foldRow,
  loadInfo,
  loadLibrary,
  markRow,
  pointRow,
  run,
  setFilter,
  setStatus,
  typeFilter,
} from './actions';
import { useApp } from './store';
import * as treeDom from './treeDom';
import { rowByKey } from './tree';

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
  needs: 'needs',
  rough: `track${sep}rough`,
  fairuz: `artist${sep}فيروز`,
  coltrane: `artist${sep}John Coltrane`,
  miles: `artist${sep}Miles Davis`,
  kindOfBlue: `album${sep}Miles Davis${sep}Kind of Blue`,
  soWhat: `track${sep}sowhat`,
  blueInGreen: `track${sep}blue`,
  nina: `artist${sep}Nina Simone`,
  singles: `singles${sep}Nina Simone`,
};

function showSample(cursor: string | null = keys.playlists, extra: Partial<ReturnType<typeof useApp.getState>> = {}): void {
  useApp.setState({ lib: sampleLibrary(), cursor, ...extra });
}

function cursorLabel(): string | undefined {
  const { lib, cursor } = useApp.getState();
  return rowByKey(lib, cursor)?.label;
}

function requestedUrls(): string[] {
  const fetchSpy = vi.mocked(fetch);
  return fetchSpy.mock.calls.map((_, i) => requestOf(fetchSpy, i).url);
}

const timers: { toFake: ('setTimeout' | 'clearTimeout')[] } = { toFake: ['setTimeout', 'clearTimeout'] };

function peekUrl(key: string, filter = ''): string {
  return `/api/peek?${new URLSearchParams(filter === '' ? { key } : { key, q: filter }).toString()}`;
}

const fetchMock = vi.fn<typeof fetch>();

beforeEach(() => {
  vi.clearAllMocks();
  vi.stubGlobal('localStorage', memoryStorage());
  useApp.setState(useApp.getInitialState(), true);
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('status', () => {
  it('setStatus puts the text and its kind in the state', () => {
    setStatus('Saved', 'good');
    expect(useApp.getState().status).toEqual({ text: 'Saved', kind: 'good' });

    setStatus('Ready');
    expect(useApp.getState().status).toEqual({ text: 'Ready', kind: '' });
  });

  it('fail shows the message of an error as a bad status', () => {
    fail(new ApiError('no such track', 422));
    expect(useApp.getState().status).toEqual({ text: 'no such track', kind: 'bad' });

    fail('plain text');
    expect(useApp.getState().status).toEqual({ text: 'plain text', kind: 'bad' });
  });
});

describe('loadInfo', () => {
  it('stores what the server says about itself', async () => {
    fetchMock.mockResolvedValue(jsonResponse(infoView({ root: '/tmp/lib' })));

    await loadInfo();

    expect(requestOf(fetchMock).url).toBe('/api/info');
    expect(useApp.getState().info?.root).toBe('/tmp/lib');
  });

  it('shows the error when the server cannot be reached', async () => {
    fetchMock.mockRejectedValue(new TypeError('Failed to fetch'));

    await loadInfo();

    expect(useApp.getState().info).toBeNull();
    expect(useApp.getState().status).toEqual({ text: 'Cannot reach mm. Is it still running?', kind: 'bad' });
  });
});

describe('loadLibrary', () => {
  it('asks for the whole library when there is no filter or sort', async () => {
    fetchMock.mockResolvedValue(jsonResponse(libraryResponse('s1')));

    await loadLibrary();

    expect(requestOf(fetchMock).url).toBe('/api/library');
    expect(useApp.getState().lib?.stamp).toBe('s1');
  });

  it('asks with the trimmed filter and the sort', async () => {
    useApp.setState({ filter: '  bach ', sort: { key: 'title', reverse: true } });
    fetchMock.mockResolvedValue(jsonResponse(libraryResponse('s1')));

    await loadLibrary();

    expect(requestOf(fetchMock).url).toBe('/api/library?q=bach&sort=title&reverse=1');
  });

  it('does not ask for reverse without a sort key', async () => {
    useApp.setState({ sort: { key: '', reverse: true } });
    fetchMock.mockResolvedValue(jsonResponse(libraryResponse('s1')));

    await loadLibrary();

    expect(requestOf(fetchMock).url).toBe('/api/library');
  });

  it('takes the sort the server answered with', async () => {
    const answer = libraryResponse('s1');
    answer.view.sort = { ...answer.view.sort, key: 'added', reverse: true };
    fetchMock.mockResolvedValue(jsonResponse(answer));

    await loadLibrary();

    expect(useApp.getState().sort).toEqual({ key: 'added', reverse: true });
  });

  it('ignores an older answer that arrives after a newer one', async () => {
    const resolvers: ((response: Response) => void)[] = [];
    fetchMock.mockImplementation(
      () =>
        new Promise<Response>((resolve) => {
          resolvers.push(resolve);
        }),
    );

    const older = loadLibrary();
    const newer = loadLibrary();
    resolvers[1]?.(jsonResponse(libraryResponse('newer')));
    await newer;
    resolvers[0]?.(jsonResponse(libraryResponse('older')));
    await older;

    expect(useApp.getState().lib?.stamp).toBe('newer');
  });

  it('shows the error and keeps the library it had', async () => {
    useApp.setState({ lib: libraryResponse('kept') });
    fetchMock.mockResolvedValue(jsonResponse({ error: 'the records are locked' }, 500));

    await loadLibrary();

    expect(useApp.getState().lib?.stamp).toBe('kept');
    expect(useApp.getState().status).toEqual({ text: 'the records are locked', kind: 'bad' });
  });
});

describe('run', () => {
  it('open goes into the fields of a track and previousTabOrBack goes back to the tree', () => {
    showSample(keys.rough, { expanded: new Set([keys.needs]) });

    run('open');
    expect(useApp.getState().focus).toBe('inspector');

    run('previousTabOrBack');
    expect(useApp.getState().focus).toBe('tree');
  });

  it('toggleFocus goes to the other pane', () => {
    showSample(keys.rough, { expanded: new Set([keys.needs]) });

    run('toggleFocus');
    expect(useApp.getState().focus).toBe('inspector');

    run('toggleFocus');
    expect(useApp.getState().focus).toBe('tree');
  });

  it('escape goes back from the fields to the tree and leaves the tree as it is', () => {
    useApp.setState({ focus: 'inspector' });

    run('escape');
    expect(useApp.getState().focus).toBe('tree');

    run('escape');
    expect(useApp.getState().focus).toBe('tree');
  });

  it('any key wipes the message in the status line', () => {
    setStatus('Saved', 'good');

    run('moveDown');

    expect(useApp.getState().status).toEqual({ text: '', kind: '' });
  });
});

describe('the cursor', () => {
  beforeEach(() => {
    vi.useFakeTimers(timers);
    stubServer(infoView(), sampleLibrary());
    showSample();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('moveDown and moveUp step through the rows that are shown', () => {
    run('moveDown');
    expect(cursorLabel()).toBe('Needs work');

    run('moveDown');
    expect(cursorLabel()).toBe('فيروز');

    run('moveUp');
    expect(cursorLabel()).toBe('Needs work');
  });

  it('the cursor stops at the first row and at the last', () => {
    run('moveUp');
    expect(cursorLabel()).toBe('Playlists');

    run('moveToBottom');
    expect(cursorLabel()).toBe('Nina Simone');

    run('moveDown');
    expect(cursorLabel()).toBe('Nina Simone');
  });

  it('moveToTop goes to the first row', () => {
    showSample(keys.miles);

    run('moveToTop');

    expect(cursorLabel()).toBe('Playlists');
  });

  it('pageDown and pageUp move by the number of rows the tree says fit', () => {
    run('pageDown');
    expect(cursorLabel()).toBe('John Coltrane');

    run('pageDown');
    expect(cursorLabel()).toBe('Nina Simone');

    run('pageUp');
    expect(cursorLabel()).toBe('فيروز');
  });

  it('centre asks the tree to scroll the cursor row to the middle', () => {
    run('centre');

    expect(treeDom.centreCursor).toHaveBeenCalledOnce();
  });

  it('a cursor on no row counts from the first row', () => {
    showSample(null);

    run('moveDown');

    expect(cursorLabel()).toBe('Needs work');
  });

  it('does nothing before the library has loaded: not even a new state', () => {
    useApp.setState({ lib: null, cursor: null });
    const before = useApp.getState();

    run('moveDown');
    run('moveToBottom');
    run('markToggle');
    run('markAll');
    run('enterRow');
    run('open');
    run('close');
    run('openLevel');
    run('closeAll');

    expect(useApp.getState()).toBe(before);
  });

  it('loads the peek of a group and the track of a track row once the cursor rests', async () => {
    const fetchSpy = vi.mocked(fetch);

    run('moveDown');
    await vi.advanceTimersByTimeAsync(39);
    expect(fetchSpy).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);

    expect(requestOf(fetchSpy, 0).url).toBe('/api/peek?key=needs');
    expect(useApp.getState().detail).toMatchObject({ kind: 'group', label: 'Needs work' });

    showSample(keys.fairuz, { expanded: new Set([keys.fairuz, `album${sep}فيروز${sep}Songs`]) });
    run('moveDown');
    await vi.advanceTimersByTimeAsync(40);

    expect(requestOf(fetchSpy, 1).url).toBe(peekUrl(`album${sep}فيروز${sep}Songs`));
    run('moveDown');
    await vi.advanceTimersByTimeAsync(40);
    expect(requestOf(fetchSpy, 2).url).toBe('/api/tracks/watani');
    expect(useApp.getState().detail).toMatchObject({ id: 'watani', title: 'وطني' });
  });

  it('asks only for the row the cursor stopped on when it passes several', async () => {
    const fetchSpy = vi.mocked(fetch);

    run('moveDown');
    run('moveDown');
    run('moveDown');
    await vi.advanceTimersByTimeAsync(40);

    expect(fetchSpy).toHaveBeenCalledOnce();
    expect(requestOf(fetchSpy).url).toBe(peekUrl(keys.coltrane));
  });

  it('sends the filter with the peek of a group', async () => {
    showSample(keys.miles, { filter: ' miles ' });

    run('moveDown');
    await vi.advanceTimersByTimeAsync(40);

    expect(requestOf(vi.mocked(fetch)).url).toBe(peekUrl(keys.kindOfBlue, 'miles'));
  });

  it('pointing at the row the cursor is on drops a field that was being typed in too', () => {
    showSample(keys.miles);
    useApp.setState({ edit: { field: 1, text: 'draft' } });

    pointRow(keys.miles);

    expect(useApp.getState().edit).toBeNull();
  });

  it('moving drops a field that was being typed in', () => {
    useApp.setState({ edit: { field: 1, text: 'draft' } });

    run('moveDown');

    expect(useApp.getState().edit).toBeNull();
  });

  it('shows the error and clears the details when the details cannot be had', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse({ error: 'the track is gone' }, 404));

    run('moveDown');
    await vi.advanceTimersByTimeAsync(40);

    expect(useApp.getState().detail).toBeNull();
    expect(useApp.getState().status).toEqual({ text: 'the track is gone', kind: 'bad' });
  });
});

describe('opening and closing', () => {
  beforeEach(() => {
    vi.useFakeTimers(timers);
    stubServer(infoView(), sampleLibrary());
    showSample();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  const expanded = () => [...useApp.getState().expanded];

  it('enterRow opens a closed artist and closes it again', () => {
    showSample(keys.miles);

    run('enterRow');
    expect(expanded()).toEqual([keys.miles]);

    run('enterRow');
    expect(expanded()).toEqual([]);
  });

  it('enterRow on a track goes into its fields', () => {
    showSample(keys.rough, { expanded: new Set([keys.needs]) });

    run('enterRow');

    expect(useApp.getState().focus).toBe('inspector');
  });

  it('enterRow does not fold anything while a filter is typed or in a sorted list', () => {
    showSample(keys.miles, { filter: 'miles' });
    run('enterRow');
    expect(expanded()).toEqual([]);

    const sorted = sampleLibrary();
    sorted.view.sort = { ...sorted.view.sort, key: 'title' };
    useApp.setState({ lib: sorted, filter: '', cursor: keys.miles });
    run('enterRow');
    expect(expanded()).toEqual([]);
  });

  it('open on a closed group opens it, and on an open one steps to its first row', () => {
    run('open');
    expect(expanded()).toEqual([keys.playlists]);
    expect(useApp.getState().focus).toBe('tree');

    run('open');
    expect(cursorLabel()).toBe('Late night');
  });

  it('open on a row with fields (an artist, an album, a playlist) goes into them, starting at the first', () => {
    showSample(keys.miles, { field: 2 });

    run('open');

    expect(useApp.getState().focus).toBe('inspector');
    expect(useApp.getState().field).toBe(0);
    expect(expanded()).toEqual([]);
  });

  it('open on a track keeps the field it was on', () => {
    showSample(keys.rough, { expanded: new Set([keys.needs]), field: 2 });

    run('open');

    expect(useApp.getState().focus).toBe('inspector');
    expect(useApp.getState().field).toBe(2);
  });

  it('open on Singles, which has no fields, opens it', () => {
    showSample(keys.singles, { expanded: new Set([keys.nina]) });

    run('open');

    expect(expanded()).toContain(keys.singles);
    expect(useApp.getState().focus).toBe('tree');
  });

  it('close closes an open row, and on a closed row goes up to its parent', () => {
    showSample(keys.kindOfBlue, { expanded: new Set([keys.miles, keys.kindOfBlue]) });

    run('close');
    expect(expanded()).toEqual([keys.miles]);
    expect(cursorLabel()).toBe('Kind of Blue');

    run('close');
    expect(cursorLabel()).toBe('Miles Davis');
    expect(expanded()).toEqual([keys.miles]);

    run('close');
    expect(expanded()).toEqual([]);

    run('close');
    expect(cursorLabel()).toBe('Miles Davis');
  });

  it('close on a track goes up to its album', () => {
    showSample(keys.soWhat, { expanded: new Set([keys.miles, keys.kindOfBlue]) });

    run('close');

    expect(cursorLabel()).toBe('Kind of Blue');
  });

  it('close goes nowhere in a sorted list, where rows have no parents', () => {
    const sorted = sampleLibrary();
    sorted.view.sort = { ...sorted.view.sort, key: 'title' };
    useApp.setState({ lib: sorted, cursor: keys.soWhat });

    run('close');

    expect(useApp.getState().cursor).toBe(keys.soWhat);
  });

  it('close folds nothing while a filter is typed, but still goes up', () => {
    showSample(keys.kindOfBlue, { filter: 'miles' });

    run('close');

    expect(cursorLabel()).toBe('Miles Davis');
    expect(expanded()).toEqual([]);
  });

  it('openLevel opens the top level, then the next one', () => {
    run('openLevel');
    expect(expanded()).toEqual([keys.playlists, keys.needs, keys.fairuz, keys.coltrane, keys.miles, keys.nina]);

    run('openLevel');
    expect(expanded()).toHaveLength(6 + 5);
    expect(expanded()).toContain(keys.kindOfBlue);
  });

  it('closeAll closes everything and keeps the cursor on the row that holds it', () => {
    showSample(keys.soWhat, { expanded: new Set([keys.miles, keys.kindOfBlue]) });

    run('closeAll');

    expect(expanded()).toEqual([]);
    expect(cursorLabel()).toBe('Miles Davis');
  });

  it('the rows that are open are remembered', () => {
    vi.stubGlobal('localStorage', memoryStorage());
    showSample(keys.miles);

    run('enterRow');

    expect(localStorage.getItem('mm.expanded')).toBe(JSON.stringify([keys.miles]));
  });
});

describe('marks', () => {
  beforeEach(() => {
    vi.useFakeTimers(timers);
    stubServer(infoView(), sampleLibrary());
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  const marks = () => [...useApp.getState().marks].toSorted();

  it('markToggle marks a track and moves down to the next row', () => {
    showSample(keys.soWhat, { expanded: new Set([keys.miles, keys.kindOfBlue]) });

    run('markToggle');

    expect(marks()).toEqual(['sowhat']);
    expect(cursorLabel()).toBe('Blue in Green');
  });

  it('markToggle on an album marks all of its tracks, and again takes them away', () => {
    showSample(keys.kindOfBlue, { expanded: new Set([keys.miles]) });

    run('markToggle');
    expect(marks()).toEqual(['blue', 'sowhat']);

    showSample(keys.kindOfBlue, { expanded: new Set([keys.miles]), marks: new Set(['blue', 'sowhat']) });
    run('markToggle');
    expect(marks()).toEqual([]);
  });

  it('markToggle on an album with one track marked marks the rest', () => {
    showSample(keys.miles, { marks: new Set(['sowhat']) });

    run('markToggle');

    expect(marks()).toEqual(['blue', 'sowhat']);
  });

  it('markToggle on a row with no tracks marks nothing and stays', () => {
    const empty = sampleLibrary();
    empty.view.rows = [
      { kind: 'playlist', depth: 0, key: 'playlist\u001fEmpty', label: 'Empty' },
      { kind: 'playlist', depth: 0, key: 'playlist\u001fOther', label: 'Other' },
    ];
    useApp.setState({ lib: empty, cursor: 'playlist\u001fEmpty' });

    run('markToggle');

    expect(marks()).toEqual([]);
    expect(useApp.getState().cursor).toBe('playlist\u001fEmpty');
  });

  it('markAll marks every track shown, and again unmarks them', () => {
    showSample(keys.playlists, { expanded: new Set([keys.miles, keys.kindOfBlue]) });

    run('markAll');
    expect(marks()).toEqual(['blue', 'sowhat']);

    run('markAll');
    expect(marks()).toEqual([]);
  });

  it('markAll marks the tracks of the rows a filter left, hidden under closed rows or not', () => {
    const narrowed = sampleLibrary();
    narrowed.view.rows = narrowRows(narrowed.view.rows, 'miles');
    useApp.setState({ lib: narrowed, cursor: keys.playlists, filter: 'miles' });

    run('markAll');

    expect(marks()).toEqual(['blue', 'sowhat']);
  });

  it('markRow puts the cursor on the row, hands the tree the keys and marks it', () => {
    showSample(keys.fairuz, { focus: 'inspector', expanded: new Set([keys.miles, keys.kindOfBlue]) });

    markRow(keys.soWhat);

    expect(marks()).toEqual(['sowhat']);
    expect(useApp.getState().focus).toBe('tree');
  });
});

describe('escape', () => {
  beforeEach(() => {
    vi.useFakeTimers(timers);
    stubServer(infoView(), sampleLibrary());
    showSample();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('clears the filter first and asks for the whole library', async () => {
    useApp.setState({ filter: 'miles', marks: new Set(['blue']) });

    run('escape');

    expect(useApp.getState().filter).toBe('');
    expect([...useApp.getState().marks]).toEqual(['blue']);
    await vi.advanceTimersByTimeAsync(0);
    expect(requestOf(vi.mocked(fetch)).url).toBe('/api/library');
  });

  it('clears the marks when there is no filter', () => {
    useApp.setState({ marks: new Set(['blue', 'sowhat']) });

    run('escape');

    expect(useApp.getState().marks.size).toBe(0);
  });

  it('does nothing in a tree with no filter and no marks', () => {
    run('escape');

    expect(useApp.getState().focus).toBe('tree');
    expect(vi.mocked(fetch)).not.toHaveBeenCalled();
  });
});

describe('the filter', () => {
  beforeEach(() => {
    vi.useFakeTimers(timers);
    stubServer(infoView(), sampleLibrary());
    showSample();
  });

  afterEach(() => {
    composeFilter(false);
    vi.useRealTimers();
  });

  const libraryUrls = () => requestedUrls().filter((url) => url.startsWith('/api/library'));

  it('typeFilter puts the text in the state at once and asks the server after a short pause', async () => {
    typeFilter('mi');

    expect(useApp.getState().filter).toBe('mi');
    await vi.advanceTimersByTimeAsync(149);
    expect(libraryUrls()).toEqual([]);

    await vi.advanceTimersByTimeAsync(1);

    expect(libraryUrls()).toEqual(['/api/library?q=mi']);
  });

  it('a burst of typing is one request for the last text', async () => {
    typeFilter('m');
    await vi.advanceTimersByTimeAsync(100);
    typeFilter('mi');
    await vi.advanceTimersByTimeAsync(100);
    typeFilter('mil');
    await vi.advanceTimersByTimeAsync(150);

    expect(libraryUrls()).toEqual(['/api/library?q=mil']);
  });

  it('the server words go to the server as they were typed', async () => {
    typeFilter('status:unidentified');
    await vi.advanceTimersByTimeAsync(150);

    expect(libraryUrls()).toEqual(['/api/library?q=status%3Aunidentified']);
    expect(useApp.getState().lib?.view.rows.map((row) => row.label)).toEqual(['Needs work', 'Rough Mix']);
  });

  it('asks nothing while a composition is under way, and once when it ends', async () => {
    composeFilter(true);
    typeFilter('m');
    typeFilter('mi');
    await vi.advanceTimersByTimeAsync(1000);

    expect(libraryUrls()).toEqual([]);
    expect(useApp.getState().filter).toBe('mi');

    composeFilter(false);
    await vi.advanceTimersByTimeAsync(150);

    expect(libraryUrls()).toEqual(['/api/library?q=mi']);
  });

  it('setFilter asks at once and cancels a request that was waiting', async () => {
    typeFilter('mi');
    setFilter('');

    expect(useApp.getState().filter).toBe('');
    await vi.advanceTimersByTimeAsync(1000);

    expect(libraryUrls()).toEqual(['/api/library']);
  });

  it('focusFilter focuses and selects the box', () => {
    run('focusFilter');

    expect(treeDom.focusFilterBox).toHaveBeenCalledOnce();
  });

  it('filterAccept leaves the box for the tree and keeps the text', () => {
    useApp.setState({ filter: 'miles' });

    run('filterAccept');

    expect(treeDom.leaveFilterBox).toHaveBeenCalledOnce();
    expect(useApp.getState().filter).toBe('miles');
  });

  it('filterClear leaves the box and clears the text', async () => {
    useApp.setState({ filter: 'miles' });

    run('filterClear');

    expect(treeDom.leaveFilterBox).toHaveBeenCalledOnce();
    expect(useApp.getState().filter).toBe('');
    await vi.advanceTimersByTimeAsync(0);
    expect(libraryUrls()).toEqual(['/api/library']);
  });

  it('opening and closing is not remembered while a filter is typed', () => {
    showSample(keys.miles, { filter: 'miles', expanded: new Set([keys.miles]) });
    const before = useApp.getState().expanded;

    run('enterRow');
    run('closeAll');
    run('openLevel');

    expect(useApp.getState().expanded).toBe(before);
    expect([...before]).toEqual([keys.miles]);
  });
});

describe('actions that belong to later work', () => {
  beforeEach(() => {
    stubServer(infoView(), sampleLibrary());
    showSample(keys.miles, { marks: new Set(['blue']) });
  });

  it.each(['bulkEdit', 'remove', 'undo', 'redo', 'sync', 'add', 'identify'] as const)(
    '%s changes nothing yet',
    (action) => {
      const before = useApp.getState();

      run(action);

      expect(useApp.getState()).toBe(before);
    },
  );
});

describe('loadLibrary for the tree', () => {
  beforeEach(() => {
    vi.useFakeTimers(timers);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  function serve(library: LibraryResponse) {
    return stubServer(infoView(), library);
  }

  it('puts the cursor on the first row when there is none', async () => {
    serve(sampleLibrary());

    await loadLibrary();

    expect(cursorLabel()).toBe('Playlists');
  });

  it('keeps the cursor where it is when the row is still there', async () => {
    serve(sampleLibrary());
    useApp.setState({ cursor: keys.miles });

    await loadLibrary();

    expect(cursorLabel()).toBe('Miles Davis');
  });

  it('moves a cursor whose row is gone to the first row', async () => {
    serve(sampleLibrary());
    useApp.setState({ cursor: 'artist\u001fGone' });

    await loadLibrary();

    expect(cursorLabel()).toBe('Playlists');
  });

  it('has no cursor and no details for an empty library', async () => {
    serve(libraryResponse());
    useApp.setState({
      cursor: keys.miles,
      detail: { kind: 'artist', title: 'Artist', label: 'Miles Davis', editable: true, info: [], list: [] },
    });

    await loadLibrary();

    expect(useApp.getState().cursor).toBeNull();
    expect(useApp.getState().detail).toBeNull();
    expect(vi.mocked(fetch)).toHaveBeenCalledOnce();
  });

  it('loads the details of the row the cursor is on', async () => {
    serve(sampleLibrary());
    useApp.setState({ cursor: keys.rough, expanded: new Set([keys.needs]) });

    await loadLibrary();

    expect(requestedUrls()).toEqual(['/api/library', '/api/tracks/rough']);
    expect(useApp.getState().detail).toMatchObject({ id: 'rough' });
  });

  it('forgets the marks of tracks that are no longer in the library', async () => {
    serve(sampleLibrary());
    useApp.setState({ marks: new Set(['blue', 'gone']) });

    await loadLibrary();

    expect([...useApp.getState().marks]).toEqual(['blue']);
  });

  it('forgets the marks of tracks that are gone from a sorted list too', async () => {
    const sorted = sampleLibrary();
    sorted.view.rows = sorted.view.rows.filter((row) => row.kind === 'track').map((row) => ({ ...row, depth: 0 }));
    sorted.view.sort = { ...sorted.view.sort, key: 'title' };
    serve(sorted);
    useApp.setState({ sort: { key: 'title', reverse: false }, marks: new Set(['blue', 'gone']) });

    await loadLibrary();

    expect([...useApp.getState().marks]).toEqual(['blue']);
  });

  it('keeps the marks while a filter shows only some of the tracks', async () => {
    serve(sampleLibrary());
    useApp.setState({ filter: 'miles', marks: new Set(['blue', 'giant']) });

    await loadLibrary();

    expect([...useApp.getState().marks].toSorted()).toEqual(['blue', 'giant']);
  });

  it('follows the rows a change renamed: open stays open, the cursor stays on it', async () => {
    const renamed = sampleLibrary();
    renamed.view.rows = renamed.view.rows.map((row) =>
      row.key === keys.miles ? { ...row, key: `artist${sep}Miles Dewey Davis`, label: 'Miles Dewey Davis' } : row,
    );
    serve(renamed);
    useApp.setState({ cursor: keys.miles, expanded: new Set([keys.miles, 'other']) });

    await loadLibrary({ moves: { [keys.miles]: `artist${sep}Miles Dewey Davis` } });

    expect([...useApp.getState().expanded].toSorted()).toEqual(['other', `artist${sep}Miles Dewey Davis`].toSorted());
    expect(cursorLabel()).toBe('Miles Dewey Davis');
  });

  it('puts the cursor on the track to reveal and opens the rows above it', async () => {
    serve(sampleLibrary());
    useApp.setState({ cursor: keys.playlists });

    await loadLibrary({ reveal: 'sowhat' });

    expect(cursorLabel()).toBe('So What');
    expect([...useApp.getState().expanded].toSorted()).toEqual([keys.kindOfBlue, keys.miles].toSorted());
  });

  it('keeps a cursor that is already on the track to reveal, in a playlist say', async () => {
    serve(sampleLibrary());
    const inPlaylist = `pltrack${sep}Late night${sep}blue`;
    useApp.setState({ cursor: inPlaylist, expanded: new Set([keys.playlists, keys.lateNight]) });

    await loadLibrary({ reveal: 'blue' });

    expect(useApp.getState().cursor).toBe(inPlaylist);
    expect([...useApp.getState().expanded].toSorted()).toEqual([keys.lateNight, keys.playlists].toSorted());
  });

  it('ignores a track to reveal that is not in the rows', async () => {
    serve(sampleLibrary());
    useApp.setState({ cursor: keys.miles });

    await loadLibrary({ reveal: 'nobody' });

    expect(cursorLabel()).toBe('Miles Davis');
  });

  it('takes the cursor off a row that the answer hides inside a closed row', async () => {
    serve(sampleLibrary());
    useApp.setState({ cursor: keys.soWhat, expanded: new Set() });

    await loadLibrary();

    expect(cursorLabel()).toBe('Miles Davis');
  });
});

describe('details that arrive late', () => {
  let answers: ((response: Response) => void)[];

  async function settle(): Promise<void> {
    for (let turn = 0; turn < 10; turn += 1) {
      await new Promise<void>((resolve) => {
        setImmediate(resolve);
      });
    }
  }

  function peekAnswer(key: string): Response {
    const row = rowByKey(sampleLibrary(), key);
    if (!row) {
      throw new Error(`the sample has no row ${key}`);
    }
    return jsonResponse(peekViewFor(row));
  }

  async function moveTwice(): Promise<void> {
    run('moveDown');
    await vi.advanceTimersByTimeAsync(40);
    run('moveDown');
    await vi.advanceTimersByTimeAsync(40);
    expect(answers).toHaveLength(2);
  }

  beforeEach(() => {
    vi.useFakeTimers(timers);
    answers = [];
    vi.stubGlobal(
      'fetch',
      vi.fn<typeof fetch>(
        () =>
          new Promise<Response>((resolve) => {
            answers.push(resolve);
          }),
      ),
    );
    showSample();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('an answer for a row the cursor has left does not replace the answer for the row it is on', async () => {
    await moveTwice();

    answers[1]?.(peekAnswer(keys.fairuz));
    await settle();
    expect(useApp.getState().detail).toMatchObject({ label: 'فيروز' });

    answers[0]?.(peekAnswer(keys.needs));
    await settle();

    expect(useApp.getState().detail).toMatchObject({ label: 'فيروز' });
  });

  it('a failure for a row the cursor has left neither clears the details nor shows an error', async () => {
    await moveTwice();
    answers[1]?.(peekAnswer(keys.fairuz));
    await settle();

    answers[0]?.(jsonResponse({ error: 'the group is gone' }, 500));
    await settle();

    expect(useApp.getState().detail).toMatchObject({ label: 'فيروز' });
    expect(useApp.getState().status).toEqual({ text: '', kind: '' });
  });

  it('a failure for the row the cursor is on does show', async () => {
    await moveTwice();

    answers[1]?.(jsonResponse({ error: 'the group is gone' }, 500));
    await settle();

    expect(useApp.getState().detail).toBeNull();
    expect(useApp.getState().status).toEqual({ text: 'the group is gone', kind: 'bad' });
  });
});

describe('the mouse actions', () => {
  beforeEach(() => {
    vi.useFakeTimers(timers);
    stubServer(infoView(), sampleLibrary());
    showSample();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('pointRow puts the cursor on the row and gives the tree the keys', () => {
    useApp.setState({ focus: 'inspector' });

    pointRow(keys.coltrane);

    expect(cursorLabel()).toBe('John Coltrane');
    expect(useApp.getState().focus).toBe('tree');
  });

  it('foldRow puts the cursor on the row and opens it, then closes it', () => {
    foldRow(keys.miles);
    expect([...useApp.getState().expanded]).toEqual([keys.miles]);
    expect(cursorLabel()).toBe('Miles Davis');

    foldRow(keys.miles);
    expect([...useApp.getState().expanded]).toEqual([]);
  });
});

describe('actions that open a dialog', () => {
  beforeEach(() => {
    stubServer(infoView(), sampleLibrary());
  });

  it('help opens the help and sortMenu the sort menu', () => {
    showSample(keys.miles);

    run('help');
    expect(useApp.getState().overlay).toEqual({ kind: 'help' });

    useApp.setState({ overlay: null });
    run('sortMenu');
    expect(useApp.getState().overlay).toEqual({ kind: 'sort' });
  });

  it('addToPlaylist opens the picker for the marked tracks, or else for what the cursor is on', () => {
    showSample(keys.miles, { marks: new Set(['blue']) });

    run('addToPlaylist');
    expect(useApp.getState().overlay).toEqual({ kind: 'playlist', ids: ['blue'] });

    useApp.setState({ overlay: null, marks: new Set() });
    run('addToPlaylist');
    expect(useApp.getState().overlay).toEqual({ kind: 'playlist', ids: ['sowhat', 'blue'] });
  });

  it('copy in the fields changes nothing yet', () => {
    showSample(keys.miles);
    useApp.setState({ focus: 'inspector' });
    const before = useApp.getState();

    run('copy');

    expect(useApp.getState()).toBe(before);
  });
});
