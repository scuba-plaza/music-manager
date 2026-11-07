import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { CandidateView, SearchView } from '../api/types.gen';
import { changeResponse, infoView, jsonResponse, memoryStorage, requestsTo, sampleLibrary, slowAnswer, stubServer, type StubRoute , urlOf } from '../testing/fakes';
import {
  applyCandidate,
  cancelSearchEditing,
  chooseCandidate,
  editSearch,
  identify,
  moveCandidate,
  searchFor,
  skipCover,
  useIdentify,
} from './identify';
import { useApp } from './store';

const sep = '\u001f';
const rough = `track${sep}rough`;
const miles = `artist${sep}Miles Davis`;

let fetchMock: ReturnType<typeof stubServer>;

function candidate(text: string): CandidateView {
  return { text, diff: [{ label: 'Title', old: 'Rough Mix', new: 'So What', same: false }] };
}

function found(over: Partial<SearchView> = {}): SearchView {
  return { search: 's1', title: 'Rough Mix', artist: 'Basement Tapes', candidates: [candidate('So What · Miles Davis'), candidate('So What · Kind of Blue')], ...over };
}

function serve(route: StubRoute = () => undefined, lookup = true): void {
  fetchMock = stubServer(infoView({ lookup }), sampleLibrary(), { route });
  useApp.setState({ lib: sampleLibrary(), info: infoView({ lookup }) });
}

function deferred(): { promise: Promise<Response>; resolve: (res: Response) => void } {
  let resolve: (res: Response) => void = () => undefined;
  const promise = new Promise<Response>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

function candidatesRoute(answer: Response | Promise<Response>): StubRoute {
  return ({ method, url }) => (method === 'POST' && url.pathname === '/api/tracks/rough/candidates' ? answer : undefined);
}

function signalOf(path: string, index = 0): AbortSignal | null | undefined {
  const calls = fetchMock.mock.calls.filter(([input]) => urlOf(input) === path);
  return calls[index]?.[1]?.signal;
}

beforeEach(() => {
  vi.stubGlobal('localStorage', memoryStorage());
  useApp.setState(useApp.getInitialState(), true);
  useIdentify.setState(useIdentify.getInitialState(), true);
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('i', () => {
  it('opens the identify dialog on the track under the cursor and lists what MusicBrainz knows', async () => {
    serve(candidatesRoute(jsonResponse(found())));
    useApp.setState({ cursor: rough });

    identify();

    expect(useApp.getState().overlay).toEqual({ kind: 'identify' });
    expect(useIdentify.getState()).toMatchObject({ phase: 'searching', id: 'rough' });
    await vi.waitFor(() => {
      expect(useIdentify.getState().phase).toBe('listing');
    });
    expect(requestsTo(fetchMock, 'POST', '/api/tracks/rough/candidates')).toEqual([{}]);
    expect(useIdentify.getState()).toMatchObject({ words: { title: 'Rough Mix', artist: 'Basement Tapes' }, found: 's1', cursor: 0, error: '' });
    expect(useIdentify.getState().candidates.map((c) => c.text)).toEqual(['So What · Miles Davis', 'So What · Kind of Blue']);
  });

  it('does nothing on a row that is not a track', () => {
    serve();
    useApp.setState({ cursor: miles });

    identify();

    expect(useApp.getState().overlay).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('says so when no MusicBrainz client is set up', () => {
    serve(() => undefined, false);
    useApp.setState({ cursor: rough });

    identify();

    expect(useApp.getState().overlay).toBeNull();
    expect(useApp.getState().status).toEqual({ text: 'Lookup is not available: no MusicBrainz client is set up', kind: 'bad' });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('says why a search came to nothing, and lets the person search with other words', async () => {
    serve(candidatesRoute(jsonResponse({ error: 'No match found for “Rough Mix – Basement Tapes”.' }, 422)));
    useApp.setState({ cursor: rough });

    identify();
    await vi.waitFor(() => {
      expect(useIdentify.getState().phase).toBe('failed');
    });

    expect(useIdentify.getState().error).toBe('No match found for “Rough Mix – Basement Tapes”.');
    expect(useApp.getState().overlay).toEqual({ kind: 'identify' });
  });

  it('treats an answer with no candidates as a failure rather than showing an empty list', async () => {
    serve(candidatesRoute(jsonResponse(found({ candidates: [] }))));
    useApp.setState({ cursor: rough });

    identify();
    await vi.waitFor(() => {
      expect(useIdentify.getState().phase).toBe('failed');
    });

    expect(useIdentify.getState().error).toBe('Nothing found on MusicBrainz for “Rough Mix – Basement Tapes”');
  });
});

describe('searching with other words', () => {
  async function listing(): Promise<void> {
    useApp.setState({ cursor: rough });
    identify();
    await vi.waitFor(() => {
      expect(useIdentify.getState().phase).toBe('listing');
    });
  }

  it('sends the trimmed words and shows what the server searched for', async () => {
    let asked = 0;
    serve(({ method, url }) => {
      if (method === 'POST' && url.pathname === '/api/tracks/rough/candidates') {
        asked += 1;
        return jsonResponse(asked === 1 ? found() : found({ search: 's2', title: 'So What', artist: 'Miles Davis', candidates: [candidate('So What · Miles Davis')] }));
      }
      return undefined;
    });
    await listing();

    editSearch();
    expect(useIdentify.getState().phase).toBe('editing');
    searchFor('  So What ', ' Miles Davis  ');
    expect(useIdentify.getState().phase).toBe('searching');
    await vi.waitFor(() => {
      expect(useIdentify.getState().phase).toBe('listing');
    });

    expect(requestsTo(fetchMock, 'POST', '/api/tracks/rough/candidates')).toEqual([{}, { query: { title: 'So What', artist: 'Miles Davis' } }]);
    expect(useIdentify.getState()).toMatchObject({ words: { title: 'So What', artist: 'Miles Davis' }, found: 's2', cursor: 0 });
  });

  it('an answer to an earlier search is dropped, and the earlier search is cancelled', async () => {
    const pending = [deferred(), deferred()];
    let asked = 0;
    serve(({ method, url }) => {
      if (method === 'POST' && url.pathname === '/api/tracks/rough/candidates') {
        asked += 1;
        return pending[asked - 1]?.promise;
      }
      return undefined;
    });
    useApp.setState({ cursor: rough });
    identify();
    searchFor('So What', 'Miles Davis');

    expect(signalOf('/api/tracks/rough/candidates', 0)?.aborted).toBe(true);
    expect(signalOf('/api/tracks/rough/candidates', 1)?.aborted).toBe(false);
    pending[1]?.resolve(jsonResponse(found({ search: 'new', candidates: [candidate('new')] })));
    await vi.waitFor(() => {
      expect(useIdentify.getState().found).toBe('new');
    });
    pending[0]?.resolve(jsonResponse(found({ search: 'old', candidates: [candidate('old')] })));
    await new Promise((resolve) => setTimeout(resolve, 10));

    expect(useIdentify.getState()).toMatchObject({ found: 'new', phase: 'listing' });
  });

  it('closing the dialog cancels the search that is on its way', () => {
    serve(candidatesRoute(deferred().promise));
    useApp.setState({ cursor: rough });
    identify();

    useApp.setState({ overlay: null });

    expect(signalOf('/api/tracks/rough/candidates')?.aborted).toBe(true);
  });

  it('esc in the search form goes back to the list, or to the failure when there is no list', async () => {
    serve(candidatesRoute(jsonResponse(found())));
    await listing();
    editSearch();
    cancelSearchEditing();
    expect(useIdentify.getState().phase).toBe('listing');

    useIdentify.setState({ candidates: [], phase: 'editing' });
    cancelSearchEditing();
    expect(useIdentify.getState().phase).toBe('failed');
  });

  it('the cursor moves through the candidates and stays inside the list', async () => {
    serve(candidatesRoute(jsonResponse(found())));
    await listing();

    moveCandidate(-1);
    expect(useIdentify.getState().cursor).toBe(0);
    moveCandidate(1);
    moveCandidate(1);
    expect(useIdentify.getState().cursor).toBe(1);
    chooseCandidate(0);
    expect(useIdentify.getState().cursor).toBe(0);
    chooseCandidate(9);
    expect(useIdentify.getState().cursor).toBe(1);
  });
});

describe('applying a candidate', () => {
  const apply: StubRoute = ({ method, url }) => (method === 'POST' && url.pathname === '/api/identify/apply' ? jsonResponse(changeResponse({ message: 'Identified So What', reveal: 'rough' })) : undefined);

  async function listing(route: StubRoute): Promise<void> {
    serve(({ method, url, body }) => candidatesRoute(jsonResponse(found()))({ method, url, body }) ?? route({ method, url, body }));
    useApp.setState({ cursor: rough });
    identify();
    await vi.waitFor(() => {
      expect(useIdentify.getState().phase).toBe('listing');
    });
  }

  it('sends the search and the chosen candidate, then closes the dialog and shows the track', async () => {
    await listing(apply);
    chooseCandidate(1);

    await applyCandidate();

    expect(requestsTo(fetchMock, 'POST', '/api/identify/apply')).toEqual([{ search: 's1', index: 1 }]);
    expect(useApp.getState().overlay).toBeNull();
    expect(useApp.getState().status).toEqual({ text: 'Identified So What', kind: '' });
    expect(useApp.getState().busy).toBe(false);
  });

  it('keeps the dialog open and says what went wrong when the server refuses', async () => {
    await listing(({ method, url }) => (method === 'POST' && url.pathname === '/api/identify/apply' ? jsonResponse({ error: 'The search is too old' }, 422) : undefined));

    await applyCandidate();

    expect(useIdentify.getState()).toMatchObject({ phase: 'failed', error: 'The search is too old' });
    expect(useApp.getState().overlay).toEqual({ kind: 'identify' });
    expect(useApp.getState().busy).toBe(false);
  });

  it('goes back to the list when another change is on its way', async () => {
    await listing(apply);
    useApp.setState({ busy: true });

    await applyCandidate();

    expect(useIdentify.getState().phase).toBe('listing');
    expect(requestsTo(fetchMock, 'POST', '/api/identify/apply')).toEqual([]);
  });

  it('esc while the cover is fetched identifies the track without it, and shows the track a moment later', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    const pending = deferred();
    await listing(({ method, url }) => (method === 'POST' && url.pathname === '/api/identify/apply' ? pending.promise : undefined));
    const applying = applyCandidate();
    expect(useIdentify.getState().phase).toBe('applying');

    skipCover();
    await applying;

    expect(signalOf('/api/identify/apply')?.aborted).toBe(true);
    expect(useApp.getState().overlay).toBeNull();
    expect(useApp.getState().status).toEqual({ text: 'Identified without waiting for the cover', kind: '' });
    const reloads = () => fetchMock.mock.calls.filter(([input]) => urlOf(input).startsWith('/api/library')).length;
    const before = reloads();
    await vi.advanceTimersByTimeAsync(599);
    expect(reloads()).toBe(before);
    await vi.advanceTimersByTimeAsync(1);
    expect(reloads()).toBe(before + 1);
    expect(useApp.getState().busy).toBe(false);
  });
});

describe('answers that were already on their way when something else began', () => {
  it('an earlier search that was read late does not replace the later one', async () => {
    const slow = slowAnswer();
    let asked = 0;
    serve(({ method, url }) => {
      if (method === 'POST' && url.pathname === '/api/tracks/rough/candidates') {
        asked += 1;
        return asked === 1 ? slow.response : jsonResponse(found({ search: 'new', candidates: [candidate('new')] }));
      }
      return undefined;
    });
    useApp.setState({ cursor: rough });
    identify();
    await vi.waitFor(() => {
      expect(asked).toBe(1);
    });

    searchFor('So What', 'Miles Davis');
    await vi.waitFor(() => {
      expect(useIdentify.getState().found).toBe('new');
    });
    slow.finish(found({ search: 'old', candidates: [candidate('old')] }));
    await new Promise((resolve) => setTimeout(resolve, 10));

    expect(useIdentify.getState()).toMatchObject({ found: 'new', phase: 'listing' });
  });

  it('an apply that was read late does not close a dialog opened since', async () => {
    const slow = slowAnswer();
    serve(({ method, url }) => {
      if (method === 'POST' && url.pathname === '/api/tracks/rough/candidates') {
        return jsonResponse(found());
      }
      return method === 'POST' && url.pathname === '/api/identify/apply' ? slow.response : undefined;
    });
    useApp.setState({ cursor: rough });
    identify();
    await vi.waitFor(() => {
      expect(useIdentify.getState().phase).toBe('listing');
    });
    const applying = applyCandidate();
    await vi.waitFor(() => {
      expect(fetchMock.mock.calls.some(([input]) => urlOf(input) === '/api/identify/apply')).toBe(true);
    });

    skipCover();
    identify();
    slow.finish(changeResponse({ message: 'Identified So What' }));
    await applying;

    expect(useApp.getState().overlay).toEqual({ kind: 'identify' });
  });
});
