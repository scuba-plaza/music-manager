import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { JobView, OpCounts, PlanView, SyncData, TargetView } from '../api/types.gen';
import { infoView, jsonResponse, memoryStorage, requestsTo, sampleLibrary, stubServer, type StubRoute, urlOf } from '../testing/fakes';
import { run } from './actions';
import { jobEvent } from './jobs';
import { useApp } from './store';
import {
  applyPlan,
  backToChoosing,
  backToReview,
  chooseProfile,
  chooseTarget,
  confirmDelete,
  cycleProfile,
  moveTarget,
  openSync,
  pickTarget,
  progressOf,
  submitFolder,
  togglePrune,
  typeConfirm,
  typeFolder,
  useSync,
} from './sync';

let fetchMock: ReturnType<typeof stubServer>;

const targets: TargetView[] = [
  { label: 'Library folder', path: '~/Music', kind: 'library' },
  { label: 'USB', path: '/media/me/USB', kind: 'drive' },
];

const profiles = [
  { name: 'generic', label: 'Generic', builtin: true },
  { name: 'wolfgang-wmp01', label: 'Wolfgang WMP01', builtin: true },
  { name: 'my-player', label: 'My Player', builtin: false },
];

function counts(over: Partial<OpCounts> = {}): OpCounts {
  return { create: 1, update: 0, stray: 0, remove: 0, ...over };
}

function plan(over: Partial<PlanView> = {}): PlanView {
  return { plan: 'p1', where: '~/player', prune: false, ops: [{ kind: 'create', path: 'a.mp3' }], counts: counts(), text: '1 new', warnings: [], ...over };
}

function job(state: string, data?: SyncData, error?: string): JobView {
  return { id: 'j1', kind: 'sync', state, ...(data === undefined ? {} : { data }), ...(error === undefined ? {} : { error }) };
}

function planned(over: Partial<PlanView> = {}): JobView {
  return job('done', { phase: 'plan', plan: plan(over) });
}

const served: StubRoute = ({ method, url }) => {
  if (method === 'GET' && url.pathname === '/api/sync/targets') {
    return jsonResponse({ targets, wolfgang: false });
  }
  if (method === 'POST' && url.pathname === '/api/sync/plan') {
    return jsonResponse(job('running'));
  }
  if (method === 'POST' && url.pathname === '/api/sync/p1/apply') {
    return jsonResponse(job('running'));
  }
  if (method === 'DELETE' && url.pathname === '/api/jobs/j1') {
    return jsonResponse(job('stopped'));
  }
  return undefined;
};

function routes(...all: StubRoute[]): StubRoute {
  return (request) => {
    for (const route of all) {
      const answer = route(request);
      if (answer !== undefined) {
        return answer;
      }
    }
    return undefined;
  };
}

function serve(...extra: StubRoute[]): void {
  fetchMock = stubServer(infoView(), sampleLibrary(), { route: routes(...extra, served) });
  useApp.setState({ lib: sampleLibrary(), profiles });
}

function state() {
  return useSync.getState();
}

function libraryReloads(): number {
  return fetchMock.mock.calls.filter(([input]) => urlOf(input).startsWith('/api/library')).length;
}

async function opened(...extra: StubRoute[]): Promise<void> {
  serve(...extra);
  await openSync();
}

async function planning(at: number, ...extra: StubRoute[]): Promise<void> {
  await opened(...extra);
  chooseTarget(at);
  if (at === 2) {
    typeFolder('~/player');
    submitFolder();
  } else {
    pickTarget();
  }
  await vi.waitFor(() => {
    expect(state().job?.id).toBe('j1');
  });
}

async function reviewing(over: Partial<PlanView> = {}, ...extra: StubRoute[]): Promise<void> {
  await planning(2, ...extra);
  jobEvent(planned(over));
  await vi.waitFor(() => {
    expect(state().phase).toBe('reviewing');
  });
}

async function applying(over: Partial<PlanView> = {}, ...extra: StubRoute[]): Promise<void> {
  await reviewing(over, ...extra);
  applyPlan();
  await vi.waitFor(() => {
    expect(state().phase).toBe('applying');
    expect(state().job?.state).toBe('running');
  });
}

function sent(path: string): unknown[] {
  return requestsTo(fetchMock, 'POST', path);
}

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] });
  vi.stubGlobal('localStorage', memoryStorage());
  useApp.setState(useApp.getInitialState(), true);
  useSync.setState(useSync.getInitialState(), true);
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe('s', () => {
  it('opens the dialog on the targets, with another folder at the end and generic as the format', async () => {
    await opened();

    expect(useApp.getState().overlay).toEqual({ kind: 'sync' });
    expect(state()).toMatchObject({ phase: 'choosing', cursor: 0, profile: 'generic', folder: '', error: '', plan: null, job: null });
    expect(state().targets).toEqual([...targets, { label: 'Another folder…', path: '', kind: 'folder' }]);
  });

  it('is the action of the s key', async () => {
    serve();

    run('sync');

    await vi.waitFor(() => {
      expect(useApp.getState().overlay).toEqual({ kind: 'sync' });
    });
  });

  it('opens nothing before the library is there', async () => {
    serve();
    useApp.setState({ lib: null });

    await openSync();

    expect(fetchMock).not.toHaveBeenCalled();
    expect(useApp.getState().overlay).toBeNull();
  });

  it('says why when the targets cannot be had, and opens nothing', async () => {
    serve(({ method, url }) => (method === 'GET' && url.pathname === '/api/sync/targets' ? jsonResponse({ error: 'no mounts' }, 500) : undefined));

    await openSync();

    expect(useApp.getState().overlay).toBeNull();
    expect(useApp.getState().status).toEqual({ text: 'no mounts', kind: 'bad' });
  });

  it('does not take over from a dialog that opened in the meantime', async () => {
    serve();
    const opening = openSync();
    useApp.setState({ overlay: { kind: 'help' } });

    await opening;

    expect(useApp.getState().overlay).toEqual({ kind: 'help' });
    expect(state().targets).toEqual([]);
  });

  it('starts again from the standard format each time, remembering nothing', async () => {
    await opened();
    chooseProfile('my-player');
    typeFolder('~/old');
    useApp.setState({ overlay: null });

    await openSync();

    expect(state()).toMatchObject({ profile: 'generic', folder: '', cursor: 0, phase: 'choosing' });
  });
});

describe('choosing where', () => {
  beforeEach(async () => {
    await opened();
  });

  it('keeps the choice among the targets', () => {
    chooseTarget(5);
    expect(state().cursor).toBe(2);
    chooseTarget(-3);
    expect(state().cursor).toBe(0);
    moveTarget(1);
    expect(state().cursor).toBe(1);
    moveTarget(-1);
    moveTarget(-1);
    expect(state().cursor).toBe(0);
  });

  it('w goes through the profiles in order and round again', () => {
    chooseTarget(2);

    cycleProfile();
    expect(state().profile).toBe('wolfgang-wmp01');
    cycleProfile();
    expect(state().profile).toBe('my-player');
    cycleProfile();
    expect(state().profile).toBe('generic');
  });

  it('w does nothing for the library folder, whose format is not a choice', () => {
    chooseTarget(0);

    cycleProfile();

    expect(state().profile).toBe('generic');
  });

  it('w does nothing without profiles', () => {
    useApp.setState({ profiles: [] });
    chooseTarget(1);

    cycleProfile();

    expect(state().profile).toBe('generic');
  });

  it('the format can be chosen by name', () => {
    chooseProfile('my-player');

    expect(state().profile).toBe('my-player');
  });

  it('enter on another folder asks which folder instead of planning', () => {
    chooseTarget(2);
    useSync.setState({ error: 'old' });

    pickTarget();

    expect(state()).toMatchObject({ phase: 'typing', error: '' });
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('the typed folder keeps what was typed and drops an old complaint', () => {
    useSync.setState({ error: 'old' });

    typeFolder('~/player');

    expect(state()).toMatchObject({ folder: '~/player', error: '' });
  });

  it('esc in the folder goes back to the targets', () => {
    useSync.setState({ phase: 'typing', error: 'x' });

    backToChoosing();

    expect(state()).toMatchObject({ phase: 'choosing', error: '' });
  });

  it('an empty folder is not planned', () => {
    chooseTarget(2);
    pickTarget();
    typeFolder('   ');

    submitFolder();

    expect(state().error).toBe('Type the folder to write into');
    expect(state().phase).toBe('typing');
    expect(sent('/api/sync/plan')).toEqual([]);
  });
});

describe('planning', () => {
  it('the library folder is planned as the library, in the standard format whatever was chosen', async () => {
    await opened();
    chooseProfile('my-player');

    pickTarget();

    expect(state()).toMatchObject({ phase: 'planning', where: 'Library folder', error: '' });
    await vi.waitFor(() => {
      expect(sent('/api/sync/plan')).toEqual([{ library: true, folder: '', profile: '' }]);
    });
  });

  it('a drive is planned by its path with the chosen format', async () => {
    await opened();
    chooseTarget(1);
    chooseProfile('wolfgang-wmp01');

    pickTarget();

    expect(state().where).toBe('USB');
    await vi.waitFor(() => {
      expect(sent('/api/sync/plan')).toEqual([{ library: false, folder: '/media/me/USB', profile: 'wolfgang-wmp01' }]);
    });
  });

  it('a typed folder is planned as typed, and named without the spaces around it', async () => {
    await opened();
    chooseTarget(2);
    pickTarget();
    typeFolder(' ~/player ');
    chooseProfile('my-player');

    submitFolder();

    expect(state().where).toBe('~/player');
    await vi.waitFor(() => {
      expect(sent('/api/sync/plan')).toEqual([{ library: false, folder: ' ~/player ', profile: 'my-player' }]);
    });
  });

  it('never sends the old format flag', async () => {
    await planning(1);

    expect(JSON.stringify(sent('/api/sync/plan'))).not.toContain('wolfgang"');
  });

  it('shows what the job says as it goes', async () => {
    await planning(1);

    jobEvent(job('running', { phase: 'plan', done: 3, total: 9 }));

    expect(state().job?.data).toEqual({ phase: 'plan', done: 3, total: 9 });
    expect(state().phase).toBe('planning');
  });

  it('ends in the review of the plan it made', async () => {
    await planning(1);

    jobEvent(planned());

    await vi.waitFor(() => {
      expect(state()).toMatchObject({ phase: 'reviewing', plan: plan() });
    });
  });

  it('reviewing a library folder that is up to date reloads the library, because the header count goes', async () => {
    await planning(0);
    const before = libraryReloads();

    jobEvent(planned({ ops: [], counts: counts({ create: 0 }) }));

    await vi.waitFor(() => {
      expect(libraryReloads()).toBeGreaterThan(before);
    });
  });

  it('reviewing a library folder with things to write does not reload', async () => {
    await planning(0);
    const before = libraryReloads();

    jobEvent(planned());
    await vi.waitFor(() => {
      expect(state().phase).toBe('reviewing');
    });

    expect(libraryReloads()).toBe(before);
  });

  it('a drive that is up to date does not reload', async () => {
    await planning(1);
    const before = libraryReloads();

    jobEvent(planned({ ops: [], counts: counts({ create: 0 }) }));
    await vi.waitFor(() => {
      expect(state().phase).toBe('reviewing');
    });

    expect(libraryReloads()).toBe(before);
  });

  it('a plan that fails shows why', async () => {
    await planning(1);

    jobEvent(job('failed', undefined, 'the drive is read-only'));

    await vi.waitFor(() => {
      expect(state()).toMatchObject({ phase: 'failed', error: 'the drive is read-only' });
    });
  });

  it('a plan that was stopped says so', async () => {
    await planning(1);

    jobEvent(job('stopped'));

    await vi.waitFor(() => {
      expect(state()).toMatchObject({ phase: 'failed', error: 'The sync was stopped.' });
    });
  });

  it('a plan that ends without a plan is a failure, not a review of nothing', async () => {
    await planning(1);

    jobEvent(job('done', { phase: 'plan' }));

    await vi.waitFor(() => {
      expect(state().phase).toBe('failed');
    });
  });

  it('a plan the server refuses for a typed folder goes back to the folder with the reason', async () => {
    await opened(({ method, url }) => (method === 'POST' && url.pathname === '/api/sync/plan' ? jsonResponse({ error: 'that folder holds the library' }, 422) : undefined));
    chooseTarget(2);
    pickTarget();
    typeFolder('~');

    submitFolder();

    await vi.waitFor(() => {
      expect(state()).toMatchObject({ phase: 'typing', error: 'that folder holds the library' });
    });
  });

  it('a plan the server refuses for a drive is a failure with the reason', async () => {
    await opened(({ method, url }) => (method === 'POST' && url.pathname === '/api/sync/plan' ? jsonResponse({ error: 'a sync is going on already' }, 409) : undefined));
    chooseTarget(1);

    pickTarget();

    await vi.waitFor(() => {
      expect(state()).toMatchObject({ phase: 'failed', error: 'a sync is going on already' });
    });
  });

  it('closing the dialog while it plans stops the job', async () => {
    await planning(1);

    useApp.setState({ overlay: null });

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'DELETE', '/api/jobs/j1')).toEqual([null]);
    });
  });

  it('closing before the server has answered stops the job it then starts', async () => {
    await opened();
    chooseTarget(1);
    pickTarget();

    useApp.setState({ overlay: null });

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'DELETE', '/api/jobs/j1')).toEqual([null]);
    });
    expect(state().job).toBeNull();
  });

  it('a plan that arrives after the dialog was closed changes nothing', async () => {
    await planning(1);
    useApp.setState({ overlay: null });

    jobEvent(planned());
    await Promise.resolve();

    expect(state().phase).toBe('planning');
    expect(state().plan).toBeNull();
  });
});

describe('the review', () => {
  it('p asks for the strays to go and shows the plan the server then made', async () => {
    const pruned = plan({ prune: true, counts: counts({ stray: 0, remove: 1 }) });
    await reviewing({ counts: counts({ stray: 1 }) }, ({ method, url }) => (method === 'POST' && url.pathname === '/api/sync/p1/prune' ? jsonResponse(pruned) : undefined));

    await togglePrune();

    expect(sent('/api/sync/p1/prune')).toEqual([{ prune: true }]);
    expect(state().plan).toEqual(pruned);
  });

  it('p again keeps them', async () => {
    const kept = plan({ prune: false });
    await reviewing({ prune: true }, ({ method, url }) => (method === 'POST' && url.pathname === '/api/sync/p1/prune' ? jsonResponse(kept) : undefined));

    await togglePrune();

    expect(sent('/api/sync/p1/prune')).toEqual([{ prune: false }]);
    expect(state().plan).toEqual(kept);
  });

  it('a refusal to change the strays is shown and the plan stays', async () => {
    await reviewing({}, ({ method, url }) => (method === 'POST' && url.pathname === '/api/sync/p1/prune' ? jsonResponse({ error: 'that plan is gone: prepare the sync again' }, 410) : undefined));

    await togglePrune();

    expect(state().error).toBe('that plan is gone: prepare the sync again');
    expect(state().plan).toEqual(plan());
  });

  it('a change of the strays that comes back after the dialog was closed changes nothing', async () => {
    let answer: (res: Response) => void = () => undefined;
    await reviewing({}, ({ method, url }) =>
      method === 'POST' && url.pathname === '/api/sync/p1/prune'
        ? new Promise<Response>((resolve) => {
            answer = resolve;
          })
        : undefined,
    );
    const asking = togglePrune();
    useApp.setState({ overlay: null });

    answer(jsonResponse(plan({ prune: true })));
    await asking;

    expect(state().plan).toEqual(plan());
  });

  it('a change of the strays that is refused after the dialog was closed shows nothing', async () => {
    let answer: (res: Response) => void = () => undefined;
    await reviewing({}, ({ method, url }) =>
      method === 'POST' && url.pathname === '/api/sync/p1/prune'
        ? new Promise<Response>((resolve) => {
            answer = resolve;
          })
        : undefined,
    );
    const asking = togglePrune();
    useApp.setState({ overlay: null });

    answer(jsonResponse({ error: 'gone' }, 410));
    await asking;

    expect(state().error).toBe('');
  });

  it('a change that works takes the complaint about the one before away', async () => {
    let answers = 0;
    await reviewing({}, ({ method, url }) => {
      if (method === 'POST' && url.pathname === '/api/sync/p1/prune') {
        answers += 1;
        return answers === 1 ? jsonResponse({ error: 'busy' }, 500) : jsonResponse(plan({ prune: true }));
      }
      return undefined;
    });
    await togglePrune();
    expect(state().error).toBe('busy');

    await togglePrune();

    expect(state()).toMatchObject({ error: '', plan: plan({ prune: true }) });
  });

  it('there is nothing to change without a plan', async () => {
    await opened();

    await togglePrune();

    expect(sent('/api/sync/p1/prune')).toEqual([]);
  });

  it('closing the review drops the plan on the server', async () => {
    await reviewing();

    useApp.setState({ overlay: null });

    expect(requestsTo(fetchMock, 'DELETE', '/api/sync/p1')).toEqual([null]);
  });

  it('closing the confirmation drops it too', async () => {
    await reviewing({ counts: counts({ remove: 2 }) });
    applyPlan();

    useApp.setState({ overlay: null });

    expect(requestsTo(fetchMock, 'DELETE', '/api/sync/p1')).toEqual([null]);
  });

  it('closing at the targets drops nothing', async () => {
    await opened();

    useApp.setState({ overlay: null });

    expect(requestsTo(fetchMock, 'DELETE', '/api/sync/p1')).toEqual([]);
  });

  it('closing the dialog of another kind drops nothing', async () => {
    await reviewing();
    useApp.setState({ overlay: null });
    fetchMock.mockClear();

    useApp.setState({ overlay: { kind: 'help' } });
    useApp.setState({ overlay: null });

    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe('applying', () => {
  it('there is nothing to apply without a plan', async () => {
    await opened();

    applyPlan();

    expect(state().phase).toBe('choosing');
    expect(sent('/api/sync/p1/apply')).toEqual([]);
  });

  it('an up to date plan has nothing to write, so enter closes the dialog', async () => {
    await reviewing({ ops: [], counts: counts({ create: 0 }) });

    applyPlan();

    expect(useApp.getState().overlay).toBeNull();
    expect(sent('/api/sync/p1/apply')).toEqual([]);
  });

  it('a plan that deletes asks to type delete first, with the word empty', async () => {
    await reviewing({ counts: counts({ remove: 2 }) });
    useSync.setState({ confirm: 'old', error: 'old' });

    applyPlan();

    expect(state()).toMatchObject({ phase: 'confirming', confirm: '', error: '' });
    expect(sent('/api/sync/p1/apply')).toEqual([]);
  });

  it('a plan that only writes is applied at once, with no word', async () => {
    await reviewing();

    applyPlan();

    expect(state().phase).toBe('applying');
    await vi.waitFor(() => {
      expect(sent('/api/sync/p1/apply')).toEqual([{ confirm: '' }]);
    });
  });

  it('typing something else than delete starts the word again', async () => {
    await reviewing({ counts: counts({ remove: 1 }) });
    applyPlan();
    typeConfirm('delet');

    confirmDelete();

    expect(state()).toMatchObject({ phase: 'confirming', confirm: '' });
    expect(sent('/api/sync/p1/apply')).toEqual([]);
  });

  it('typing delete, with spaces around it, writes and sends the word', async () => {
    await reviewing({ counts: counts({ remove: 1 }) });
    applyPlan();
    typeConfirm(' delete ');

    confirmDelete();

    await vi.waitFor(() => {
      expect(sent('/api/sync/p1/apply')).toEqual([{ confirm: 'delete' }]);
    });
    expect(state().phase).toBe('applying');
  });

  it('esc in the confirmation goes back to the review', async () => {
    await reviewing({ counts: counts({ remove: 1 }) });
    applyPlan();
    useSync.setState({ error: 'x' });

    backToReview();

    expect(state()).toMatchObject({ phase: 'reviewing', error: '' });
  });

  it('shows how far the writing has got', async () => {
    await applying();

    jobEvent(job('running', { phase: 'apply', done: 1, total: 4 }));

    expect(state().job?.data).toEqual({ phase: 'apply', done: 1, total: 4 });
  });

  it('a server that refuses a plan with nothing to delete confirmed ends in a failure with the reason', async () => {
    await reviewing({}, ({ method, url }) => (method === 'POST' && url.pathname === '/api/sync/p1/apply' ? jsonResponse({ error: 'that plan is gone: prepare the sync again' }, 410) : undefined));

    applyPlan();

    await vi.waitFor(() => {
      expect(state()).toMatchObject({ phase: 'failed', error: 'that plan is gone: prepare the sync again' });
    });
  });

  it('a server that refuses the word goes back to the confirmation with the reason', async () => {
    await reviewing({ counts: counts({ remove: 1 }) }, ({ method, url }) => (method === 'POST' && url.pathname === '/api/sync/p1/apply' ? jsonResponse({ error: 'type delete to remove 1 file' }, 422) : undefined));
    applyPlan();
    typeConfirm('delete');

    confirmDelete();

    await vi.waitFor(() => {
      expect(state()).toMatchObject({ phase: 'confirming', error: 'type delete to remove 1 file' });
    });
  });

  it('a refusal that comes after the dialog was closed changes nothing', async () => {
    let answer: (res: Response) => void = () => undefined;
    await reviewing({}, ({ method, url }) =>
      method === 'POST' && url.pathname === '/api/sync/p1/apply'
        ? new Promise<Response>((resolve) => {
            answer = resolve;
          })
        : undefined,
    );
    applyPlan();
    useApp.setState({ overlay: null });

    answer(jsonResponse({ error: 'gone' }, 410));
    await Promise.resolve();
    await Promise.resolve();

    expect(state().phase).toBe('applying');
    expect(state().error).toBe('');
  });

  it('finishes with the summary and reloads the library, whose history and counts changed', async () => {
    await applying();
    const before = libraryReloads();

    jobEvent(job('done', { phase: 'apply', summary: '1 new' }));

    await vi.waitFor(() => {
      expect(state()).toMatchObject({ phase: 'finished', summary: '1 new' });
    });
    await vi.waitFor(() => {
      expect(libraryReloads()).toBeGreaterThan(before);
    });
    expect(useApp.getState().status.text).toBe('');
  });

  it('a write that fails shows why', async () => {
    await applying();

    jobEvent(job('failed', undefined, 'no space left'));

    await vi.waitFor(() => {
      expect(state()).toMatchObject({ phase: 'failed', error: 'no space left' });
    });
  });

  it('a write that was stopped says so', async () => {
    await applying();

    jobEvent(job('stopped'));

    await vi.waitFor(() => {
      expect(state()).toMatchObject({ phase: 'failed', error: 'The sync was stopped.' });
    });
  });

  it('a write that ends after the dialog was closed says how it went in the status line', async () => {
    await applying();
    useApp.setState({ overlay: null });

    jobEvent(job('done', { phase: 'apply', summary: '1 new' }));

    await vi.waitFor(() => {
      expect(useApp.getState().status).toEqual({ text: 'Synced to ~/player: 1 new', kind: 'good' });
    });
    expect(state().phase).toBe('applying');
  });

  it('a write that fails after the dialog was closed says so in the status line', async () => {
    await applying();
    useApp.setState({ overlay: null });

    jobEvent(job('failed', undefined, 'no space left'));

    await vi.waitFor(() => {
      expect(useApp.getState().status).toEqual({ text: 'no space left', kind: 'bad' });
    });
  });

  it('closing the dialog while it writes drops nothing and stops nothing', async () => {
    await applying();
    fetchMock.mockClear();

    useApp.setState({ overlay: null });

    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe('progressOf', () => {
  it('reads the progress of the phase asked for', () => {
    expect(progressOf(job('running', { phase: 'plan', done: 2, total: 8 }), 'plan', 0)).toEqual({ done: 2, total: 8 });
    expect(progressOf(job('running', { phase: 'apply', done: 5, total: 6 }), 'apply', 0)).toEqual({ done: 5, total: 6 });
  });

  it('knows nothing of the other phase, and starts at nothing done', () => {
    expect(progressOf(job('running', { phase: 'plan', done: 2, total: 8 }), 'apply', 7)).toEqual({ done: 0, total: 7 });
    expect(progressOf(null, 'plan', 0)).toEqual({ done: 0, total: 0 });
    expect(progressOf(job('running'), 'apply', 3)).toEqual({ done: 0, total: 3 });
  });

  it('counts nothing for a report without numbers', () => {
    expect(progressOf(job('running', { phase: 'plan' }), 'plan', 4)).toEqual({ done: 0, total: 0 });
  });

  it('ignores data that is not a report', () => {
    expect(progressOf({ id: 'j1', kind: 'sync', state: 'running', data: 'text' }, 'plan', 4)).toEqual({ done: 0, total: 4 });
    expect(progressOf({ id: 'j1', kind: 'sync', state: 'running', data: { done: 3 } }, 'plan', 4)).toEqual({ done: 0, total: 4 });
  });
});
