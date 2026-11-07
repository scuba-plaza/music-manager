import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { changeResponse, infoView, jsonResponse, memoryStorage, requestsTo, sampleLibrary, stubServer, type StubRoute } from '../testing/fakes';
import { run } from './actions';
import { useAdd } from './add';
import { useDoctor } from './doctor';
import { useIdentify } from './identify';
import { adopt, runLookup } from './lookup';
import { useApp } from './store';

const sep = '\u001f';
const keys = { rough: `track${sep}rough`, needs: 'needs', miles: `artist${sep}Miles Davis`, soWhat: `track${sep}sowhat` };

let fetchMock: ReturnType<typeof stubServer>;

const adopting: StubRoute = ({ method, url }) => (method === 'POST' && url.pathname === '/api/adopt' ? jsonResponse(changeResponse({ message: 'Identified 1 track by its old tags' })) : undefined);

function serve(route: StubRoute = adopting): void {
  fetchMock = stubServer(infoView({ lookup: true, canAdd: true }), sampleLibrary(), { route });
  useApp.setState({ lib: sampleLibrary(), info: infoView({ lookup: true, canAdd: true }) });
}

beforeEach(() => {
  vi.stubGlobal('localStorage', memoryStorage());
  useApp.setState(useApp.getInitialState(), true);
  useAdd.setState(useAdd.getInitialState(), true);
  useDoctor.setState(useDoctor.getInitialState(), true);
  useIdentify.setState(useIdentify.getInitialState(), true);
  serve();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('the keys of the library as a whole', () => {
  it('i identifies the track under the cursor', () => {
    useApp.setState({ cursor: keys.rough });

    run('identify');

    expect(useApp.getState().overlay).toEqual({ kind: 'identify' });
  });

  it('I asks about identifying everything', () => {
    run('identifyAll');

    expect(useApp.getState().overlay).toMatchObject({ kind: 'ask', yes: 'identify' });
  });

  it('a opens the add dialog', () => {
    run('add');

    expect(useApp.getState().overlay).toEqual({ kind: 'add' });
  });

  it('D opens the library check', () => {
    run('doctor');

    expect(useApp.getState().overlay).toEqual({ kind: 'doctor' });
  });

  it('answers only for these keys', () => {
    expect(runLookup('moveDown')).toBe(false);
    expect(runLookup('remove')).toBe(false);
    expect(runLookup('sync')).toBe(false);
  });
});

describe('o', () => {
  it('uses the old tags of the track under the cursor', async () => {
    useApp.setState({ cursor: keys.rough });

    adopt();

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'POST', '/api/adopt')).toEqual([{ ids: ['rough'] }]);
    });
    await vi.waitFor(() => {
      expect(useApp.getState().status.text).toBe('Identified 1 track by its old tags');
    });
  });

  it('uses the old tags of every track of the Needs work group on its row', async () => {
    useApp.setState({ cursor: keys.needs });

    run('adopt');

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'POST', '/api/adopt')).toEqual([{ ids: ['rough'] }]);
    });
  });

  it('prefers the tracks that are marked', async () => {
    useApp.setState({ cursor: keys.rough, marks: new Set(['sowhat']) });

    adopt();

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'POST', '/api/adopt')).toEqual([{ ids: ['sowhat'] }]);
    });
  });

  it('asks for a track that needs work when the cursor is on an artist', () => {
    useApp.setState({ cursor: keys.miles });

    adopt();

    expect(useApp.getState().status).toEqual({ text: 'Select a track that needs work, or the Needs work group', kind: 'bad' });
    expect(requestsTo(fetchMock, 'POST', '/api/adopt')).toEqual([]);
  });

  it('says what the server refused', async () => {
    serve(({ method, url }) => (method === 'POST' && url.pathname === '/api/adopt' ? jsonResponse({ error: 'those files have no tags' }, 422) : undefined));
    useApp.setState({ cursor: keys.rough });

    adopt();

    await vi.waitFor(() => {
      expect(useApp.getState().status).toEqual({ text: 'those files have no tags', kind: 'bad' });
    });
  });
});
