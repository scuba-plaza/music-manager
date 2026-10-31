import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { connect } from '../state/actions';
import { useApp } from '../state/store';
import { FakeEventSource, jsonResponse, libraryResponse, memoryStorage, requestOf, urlOf } from '../testing/fakes';
import { subscribe } from './events';
import type { JobView } from './types.gen';

function openStream(): FakeEventSource {
  const stream = FakeEventSource.instances[0];
  if (!stream) {
    throw new Error('no stream was opened');
  }
  return stream;
}

beforeEach(() => {
  FakeEventSource.instances = [];
  vi.stubGlobal('EventSource', FakeEventSource);
  vi.stubGlobal('localStorage', memoryStorage());
});

afterEach(() => {
  expect(FakeEventSource.instances.filter((stream) => !stream.closed)).toEqual([]);
  vi.unstubAllGlobals();
});

describe('subscribe', () => {
  function recorder() {
    const seen: string[] = [];
    return {
      seen,
      handlers: {
        library: (stamp: string) => seen.push(`library ${stamp}`),
        job: (job: JobView) => seen.push(`job ${job.id} ${job.state}`),
        open: () => seen.push('open'),
      },
    };
  }

  it('listens to the event stream of the server', () => {
    const stop = subscribe(recorder().handlers);

    expect(openStream().url).toBe('/api/events');
    stop();
  });

  it('a library event with the same stamp does nothing', () => {
    const { seen, handlers } = recorder();
    const stop = subscribe(handlers);

    openStream().emit('library', '"a1"');
    openStream().emit('library', '"a1"');
    openStream().emit('library', '"b2"');
    openStream().emit('library', '"a1"');

    expect(seen).toEqual(['library a1', 'library b2', 'library a1']);
    stop();
  });

  it('asks for the library anyway when a library event cannot be read', () => {
    const { seen, handlers } = recorder();
    const stop = subscribe(handlers);

    openStream().emit('library', 'not json');
    openStream().emit('library', 'not json');
    openStream().emit('library', '7');

    expect(seen).toEqual(['library ', 'library ', 'library ']);
    stop();
  });

  it('hands a job event to the job handler', () => {
    const { seen, handlers } = recorder();
    const stop = subscribe(handlers);

    openStream().emit('job', JSON.stringify({ id: 'j1', kind: 'sync', state: 'running', data: { phase: 'copy' } }));

    expect(seen).toEqual(['job j1 running']);
    stop();
  });

  it('drops a job event that is not a job', () => {
    const { seen, handlers } = recorder();
    const stop = subscribe(handlers);

    openStream().emit('job', 'not json');
    openStream().emit('job', '{"id":"j1"}');
    openStream().emit('job', 'null');

    expect(seen).toEqual([]);
    stop();
  });

  it('says open every time the stream opens, a reconnect included', () => {
    const { seen, handlers } = recorder();
    const stop = subscribe(handlers);

    openStream().emit('open');
    openStream().emit('open');

    expect(seen).toEqual(['open', 'open']);
    stop();
  });

  it('closes the stream when stopped', () => {
    const stop = subscribe(recorder().handlers);
    expect(openStream().closed).toBe(false);

    stop();

    expect(openStream().closed).toBe(true);
  });
});

describe('connect', () => {
  const fetchMock = vi.fn<typeof fetch>();

  function libraryRequests(): number {
    return fetchMock.mock.calls.filter(([input]) => urlOf(input).startsWith('/api/library')).length;
  }

  beforeEach(() => {
    useApp.setState(useApp.getInitialState(), true);
    fetchMock.mockReset();
    fetchMock.mockImplementation(() => Promise.resolve(jsonResponse(libraryResponse('s2'))));
    vi.stubGlobal('fetch', fetchMock);
  });

  it('reconnect after open reloads the library', async () => {
    useApp.setState({ lib: libraryResponse('s1') });
    const stop = connect(() => undefined);

    openStream().emit('open');
    await vi.waitFor(() => {
      expect(useApp.getState().lib?.stamp).toBe('s2');
    });
    expect(libraryRequests()).toBe(1);
    expect(requestOf(fetchMock).url).toBe('/api/library');

    openStream().emit('open');
    expect(libraryRequests()).toBe(2);
    stop();
  });

  it('does not load the library on open before the page has one', () => {
    const stop = connect(() => undefined);

    openStream().emit('open');

    expect(libraryRequests()).toBe(0);
    stop();
  });

  it('reloads the library for a library event with a stamp the page does not have', async () => {
    useApp.setState({ lib: libraryResponse('s1') });
    const stop = connect(() => undefined);

    openStream().emit('library', '"s2"');

    expect(libraryRequests()).toBe(1);
    await vi.waitFor(() => {
      expect(useApp.getState().lib?.stamp).toBe('s2');
    });
    stop();
  });

  it('does not reload the library for the stamp the page already has', () => {
    useApp.setState({ lib: libraryResponse('s1') });
    const stop = connect(() => undefined);

    openStream().emit('library', '"s1"');

    expect(libraryRequests()).toBe(0);
    stop();
  });

  it('does not reload the library while a change is in flight', () => {
    useApp.setState({ lib: libraryResponse('s1'), busy: true });
    const stop = connect(() => undefined);

    openStream().emit('library', '"s2"');

    expect(libraryRequests()).toBe(0);
    stop();
  });

  it('hands a job event to the job handler it was given', () => {
    const jobs: string[] = [];
    const stop = connect((job) => jobs.push(job.id));

    openStream().emit('job', JSON.stringify({ id: 'j9', kind: 'identify', state: 'done' }));

    expect(jobs).toEqual(['j9']);
    stop();
  });
});
