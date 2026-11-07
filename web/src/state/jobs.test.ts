import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { JobView } from '../api/types.gen';
import { infoView, jsonResponse, requestsTo, sampleLibrary, stubServer, type StubRoute } from '../testing/fakes';
import { follow, jobEvent, stopJob } from './jobs';

let fetchMock: ReturnType<typeof stubServer>;

function serve(route: StubRoute = () => undefined): void {
  fetchMock = stubServer(infoView(), sampleLibrary(), { route });
}

function job(state: string, extra: Partial<JobView> = {}): JobView {
  return { id: 'j1', kind: 'identify', state, ...extra };
}

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] });
  serve();
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe('following a job', () => {
  it('shows the job as it is each time the server says something, and ends with the job once it is no longer running', async () => {
    const seen: JobView[] = [];
    const ended = follow(job('running'), (v) => seen.push(v));

    jobEvent(job('running', { data: { done: 1 } }));
    jobEvent(job('done', { data: { done: 2 } }));

    await expect(ended).resolves.toEqual(job('done', { data: { done: 2 } }));
    expect(seen).toEqual([job('running'), job('running', { data: { done: 1 } }), job('done', { data: { done: 2 } })]);
  });

  it('ends at once with a job that is not running', async () => {
    const seen: JobView[] = [];

    await expect(follow(job('failed', { error: 'no network' }), (v) => seen.push(v))).resolves.toEqual(job('failed', { error: 'no network' }));
    expect(seen).toEqual([job('failed', { error: 'no network' })]);
  });

  it('ignores what the server says about other jobs', async () => {
    const seen: JobView[] = [];
    const ended = follow(job('running'), (v) => seen.push(v));

    jobEvent({ id: 'other', kind: 'identify', state: 'done' });
    jobEvent(job('stopped'));

    await expect(ended).resolves.toEqual(job('stopped'));
    expect(seen).toEqual([job('running'), job('stopped')]);
  });

  it('asks for the job every 1.5 s, in case an event was missed', async () => {
    serve(({ method, url }) => (method === 'GET' && url.pathname === '/api/jobs/j1' ? jsonResponse(job('done')) : undefined));
    const seen: JobView[] = [];
    const ended = follow(job('running'), (v) => seen.push(v));

    await vi.advanceTimersByTimeAsync(1499);
    expect(fetchMock).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);

    await expect(ended).resolves.toEqual(job('done'));
    expect(seen).toEqual([job('running'), job('done')]);
  });

  it('keeps going when a question to the server fails', async () => {
    let asked = 0;
    serve(({ method, url }) => {
      if (method === 'GET' && url.pathname === '/api/jobs/j1') {
        asked += 1;
        return asked === 1 ? jsonResponse({ error: 'busy' }, 500) : jsonResponse(job('done'));
      }
      return undefined;
    });
    const ended = follow(job('running'), () => undefined);

    await vi.advanceTimersByTimeAsync(3000);

    await expect(ended).resolves.toEqual(job('done'));
    expect(asked).toBe(2);
  });

  it('ignores an answer to a question that arrives after the job has ended', async () => {
    let answerLate: (res: Response) => void = () => undefined;
    serve(({ method, url }) =>
      method === 'GET' && url.pathname === '/api/jobs/j1'
        ? new Promise<Response>((resolve) => {
            answerLate = resolve;
          })
        : undefined,
    );
    const seen: JobView[] = [];
    const ended = follow(job('running'), (v) => seen.push(v));
    await vi.advanceTimersByTimeAsync(1500);
    expect(fetchMock).toHaveBeenCalledTimes(1);

    jobEvent(job('done'));
    await ended;
    answerLate(jsonResponse(job('running')));
    await vi.advanceTimersByTimeAsync(10);

    expect(seen).toEqual([job('running'), job('done')]);
  });

  it('stops asking once the job has ended', async () => {
    serve(({ method, url }) => (method === 'GET' && url.pathname === '/api/jobs/j1' ? jsonResponse(job('done')) : undefined));
    const ended = follow(job('running'), () => undefined);
    jobEvent(job('done'));
    await ended;

    await vi.advanceTimersByTimeAsync(10000);

    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('ignores a late event for a job that has ended', async () => {
    const seen: JobView[] = [];
    const ended = follow(job('running'), (v) => seen.push(v));
    jobEvent(job('done'));
    await ended;

    jobEvent(job('running'));

    expect(seen).toEqual([job('running'), job('done')]);
  });
});

describe('stopping a job', () => {
  it('asks the server to stop it', async () => {
    serve(({ method, url }) => (method === 'DELETE' && url.pathname === '/api/jobs/j1' ? jsonResponse(job('stopped')) : undefined));

    await stopJob('j1');

    expect(requestsTo(fetchMock, 'DELETE', '/api/jobs/j1')).toEqual([null]);
  });

  it('says nothing when the server cannot', async () => {
    serve(({ method, url }) => (method === 'DELETE' && url.pathname === '/api/jobs/j1' ? jsonResponse({ error: 'gone' }, 404) : undefined));

    await expect(stopJob('j1')).resolves.toBeUndefined();
  });

  it('asks for a job by its id, whatever is in it', async () => {
    serve(() => jsonResponse(job('stopped')));

    await stopJob('a b/c');

    expect(requestsTo(fetchMock, 'DELETE', '/api/jobs/a%20b%2Fc')).toEqual([null]);
  });
});
