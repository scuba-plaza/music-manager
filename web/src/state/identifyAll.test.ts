import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { IdentifyProgress, JobView } from '../api/types.gen';
import { infoView, jsonResponse, memoryStorage, requestsTo, sampleLibrary, stubServer, type StubRoute , urlOf } from '../testing/fakes';
import { identifyAll, progressOf, stopOrClose, useIdentifyAll } from './identifyAll';
import { jobEvent } from './jobs';
import { useApp } from './store';

let fetchMock: ReturnType<typeof stubServer>;

function progress(over: Partial<IdentifyProgress> = {}): IdentifyProgress {
  return { total: 3, done: 1, identified: 1, left: 0, failed: 0, failures: [], ...over };
}

function job(state: string, data?: unknown, extra: Partial<JobView> = {}): JobView {
  return { id: 'j1', kind: 'identify', state, ...(data === undefined ? {} : { data }), ...extra };
}

function serve(route: StubRoute = () => undefined, options: { lookup?: boolean; needWork?: number } = {}): void {
  const lib = sampleLibrary();
  const withWork = { ...lib, view: { ...lib.view, needWork: options.needWork ?? 1 } };
  fetchMock = stubServer(infoView({ lookup: options.lookup ?? true }), withWork, { route });
  useApp.setState({ lib: withWork, info: infoView({ lookup: options.lookup ?? true }) });
}

function asked(): { head: string; body: string[]; yes: string; onYes: () => void } {
  const overlay = useApp.getState().overlay;
  if (overlay?.kind !== 'ask') {
    throw new Error(`no question was asked; the overlay is ${overlay?.kind ?? 'closed'}`);
  }
  return overlay;
}

const start: StubRoute = ({ method, url }) => (method === 'POST' && url.pathname === '/api/identify-all' ? jsonResponse(job('running', progress())) : undefined);

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] });
  vi.stubGlobal('localStorage', memoryStorage());
  useApp.setState(useApp.getInitialState(), true);
  useIdentifyAll.setState(useIdentifyAll.getInitialState(), true);
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe('I', () => {
  it('asks before it starts, naming how many tracks need work', () => {
    serve(start, { needWork: 3 });

    identifyAll();

    const question = asked();
    expect(question.head).toBe('Identify 3 tracks on MusicBrainz?');
    expect(question.yes).toBe('identify');
    expect(question.body).toEqual([
      'Each one is searched for, and the best match is taken only if it is a confident one (a score of 95 or more). The rest are left alone for you to look at.',
      'This takes about a second per track, and u undoes it all in one step.',
    ]);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('says one track in the singular', () => {
    serve(start, { needWork: 1 });

    identifyAll();

    expect(asked().head).toBe('Identify 1 track on MusicBrainz?');
  });

  it('says when nothing needs work', () => {
    serve(start, { needWork: 0 });

    identifyAll();

    expect(useApp.getState().overlay).toBeNull();
    expect(useApp.getState().status).toEqual({ text: 'Nothing needs work', kind: '' });
  });

  it('says when no MusicBrainz client is set up', () => {
    serve(start, { lookup: false });

    identifyAll();

    expect(useApp.getState().overlay).toBeNull();
    expect(useApp.getState().status).toEqual({ text: 'Lookup is not available: no MusicBrainz client is set up', kind: 'bad' });
  });

  it('starts the job when the person agrees, and shows its progress as it comes', async () => {
    serve(start);
    identifyAll();

    asked().onYes();
    await vi.waitFor(() => {
      expect(useApp.getState().overlay).toEqual({ kind: 'identifyAll' });
    });
    expect(requestsTo(fetchMock, 'POST', '/api/identify-all')).toEqual([null]);
    expect(useIdentifyAll.getState().job).toEqual(job('running', progress()));

    jobEvent(job('running', progress({ done: 2, identified: 2 })));

    expect(progressOf(useIdentifyAll.getState().job)).toEqual(progress({ done: 2, identified: 2 }));
  });

  it('takes the new library when the job has ended', async () => {
    serve(start);
    identifyAll();
    asked().onYes();
    await vi.waitFor(() => {
      expect(useIdentifyAll.getState().job?.state).toBe('running');
    });
    const reloads = () => fetchMock.mock.calls.filter(([input]) => urlOf(input).startsWith('/api/library')).length;
    const before = reloads();

    jobEvent(job('done', progress({ done: 3, identified: 2, left: 1 })));

    await vi.waitFor(() => {
      expect(reloads()).toBe(before + 1);
    });
    expect(useIdentifyAll.getState().job).toEqual(job('done', progress({ done: 3, identified: 2, left: 1 })));
  });

  it('says why it did not start, and opens nothing', async () => {
    serve(({ method, url }) => (method === 'POST' && url.pathname === '/api/identify-all' ? jsonResponse({ error: 'identifying is going on already' }, 409) : undefined));
    identifyAll();

    asked().onYes();

    await vi.waitFor(() => {
      expect(useApp.getState().status).toEqual({ text: 'identifying is going on already', kind: 'bad' });
    });
    expect(useApp.getState().overlay).not.toEqual({ kind: 'identifyAll' });
  });
});

describe('esc and enter in the progress dialog', () => {
  it('ask the server to stop a job that is running, and leave the dialog open to show how it ended', async () => {
    serve(({ method, url }) => {
      if (method === 'DELETE' && url.pathname === '/api/jobs/j1') {
        return jsonResponse(job('stopped'));
      }
      return start({ method, url, body: null });
    });
    identifyAll();
    asked().onYes();
    await vi.waitFor(() => {
      expect(useIdentifyAll.getState().job?.state).toBe('running');
    });

    stopOrClose();

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'DELETE', '/api/jobs/j1')).toEqual([null]);
    });
    expect(useApp.getState().overlay).toEqual({ kind: 'identifyAll' });
  });

  it('close the dialog once the job has ended', async () => {
    serve(start);
    identifyAll();
    asked().onYes();
    await vi.waitFor(() => {
      expect(useIdentifyAll.getState().job?.state).toBe('running');
    });
    jobEvent(job('done', progress({ done: 3 })));

    stopOrClose();

    expect(useApp.getState().overlay).toBeNull();
    expect(requestsTo(fetchMock, 'DELETE', '/api/jobs/j1')).toEqual([]);
  });
});

describe('what a job reports', () => {
  it('is read as progress, and as nothing done before the first report', () => {
    expect(progressOf(null)).toEqual({ total: 0, done: 0, identified: 0, left: 0, failed: 0, failures: [] });
    expect(progressOf(job('running'))).toEqual({ total: 0, done: 0, identified: 0, left: 0, failed: 0, failures: [] });
    expect(progressOf(job('running', 'not progress'))).toEqual({ total: 0, done: 0, identified: 0, left: 0, failed: 0, failures: [] });
    expect(progressOf(job('running', progress({ failed: 1, failures: ['Rough Mix: no match'] })))).toEqual(progress({ failed: 1, failures: ['Rough Mix: no match'] }));
  });
});

describe('a report that is missing something', () => {
  it.each(['total', 'done', 'identified', 'left', 'failed', 'failures'] as const)('is not read as progress without %s', (missing) => {
    const { [missing]: removed, ...rest } = progress();
    expect(removed).toBeDefined();

    expect(progressOf(job('running', rest))).toEqual({ total: 0, done: 0, identified: 0, left: 0, failed: 0, failures: [] });
  });
});

describe('a report that has the wrong kind of value', () => {
  it.each(['total', 'done', 'identified', 'left', 'failed', 'failures'] as const)('is not read as progress when %s is text', (key) => {
    expect(progressOf(job('running', { ...progress(), [key]: 'many' }))).toEqual({ total: 0, done: 0, identified: 0, left: 0, failed: 0, failures: [] });
  });
});
