import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '../api/client';
import { infoView, jsonResponse, libraryResponse, memoryStorage, requestOf } from '../testing/fakes';
import { fail, loadInfo, loadLibrary, run, setStatus } from './actions';
import { useApp } from './store';

const fetchMock = vi.fn<typeof fetch>();

beforeEach(() => {
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
  it('open goes into the fields and previousTabOrBack goes back to the tree', () => {
    run('open');
    expect(useApp.getState().focus).toBe('inspector');

    run('previousTabOrBack');
    expect(useApp.getState().focus).toBe('tree');
  });

  it('toggleFocus goes to the other pane', () => {
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
