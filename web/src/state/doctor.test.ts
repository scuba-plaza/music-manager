import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { DoctorView } from '../api/types.gen';
import { infoView, jsonResponse, memoryStorage, sampleLibrary, slowAnswer, stubServer, type StubRoute , urlOf } from '../testing/fakes';
import { chooseProblem, goToProblem, moveProblem, openDoctor, useDoctor } from './doctor';
import { useApp } from './store';

let fetchMock: ReturnType<typeof stubServer>;

function report(): DoctorView {
  return {
    problems: [
      { severity: 'error', trackId: 'rough', title: 'Rough Mix', message: 'the file is missing' },
      { severity: 'warning', trackId: 'sowhat', title: 'So What', message: 'no cover' },
    ],
    errors: 1,
    warnings: 1,
  };
}

function serve(route: StubRoute = () => undefined): void {
  fetchMock = stubServer(infoView(), sampleLibrary(), { route });
}

const doctor = (answer: Response | Promise<Response>): StubRoute => ({ method, url }) => (method === 'GET' && url.pathname === '/api/doctor' ? answer : undefined);

function libraryReloads(): number {
  return fetchMock.mock.calls.filter(([input]) => urlOf(input).startsWith('/api/library')).length;
}

beforeEach(() => {
  vi.stubGlobal('localStorage', memoryStorage());
  useApp.setState(useApp.getInitialState(), true);
  useDoctor.setState(useDoctor.getInitialState(), true);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('D', () => {
  it('opens the library check, which is checking until the server answers', async () => {
    serve(doctor(jsonResponse(report())));

    openDoctor();

    expect(useApp.getState().overlay).toEqual({ kind: 'doctor' });
    expect(useDoctor.getState()).toMatchObject({ phase: 'checking', result: null, cursor: 0 });
    await vi.waitFor(() => {
      expect(useDoctor.getState().phase).toBe('done');
    });
    expect(useDoctor.getState().result).toEqual(report());
  });

  it('says why the check could not be made', async () => {
    serve(doctor(jsonResponse({ error: 'the library is locked' }, 500)));

    openDoctor();

    await vi.waitFor(() => {
      expect(useDoctor.getState().phase).toBe('failed');
    });
    expect(useDoctor.getState().error).toBe('the library is locked');
  });

  it('starts again at the first problem each time it is opened', async () => {
    serve(doctor(jsonResponse(report())));
    openDoctor();
    await vi.waitFor(() => {
      expect(useDoctor.getState().phase).toBe('done');
    });
    moveProblem(1);
    useApp.setState({ overlay: null });

    openDoctor();

    expect(useDoctor.getState()).toMatchObject({ phase: 'checking', result: null, cursor: 0 });
  });

  it('closing the dialog cancels the check that is on its way', () => {
    serve(doctor(new Promise<Response>(() => undefined)));
    openDoctor();

    useApp.setState({ overlay: null });

    const signal = fetchMock.mock.calls[0]?.[1]?.signal;
    expect(signal?.aborted).toBe(true);
  });

  it('an answer that arrives after the dialog was closed changes nothing', async () => {
    let answer: (res: Response) => void = () => undefined;
    serve(doctor(new Promise<Response>((resolve) => {
      answer = resolve;
    })));
    openDoctor();
    useApp.setState({ overlay: null });
    answer(jsonResponse(report()));
    await new Promise((resolve) => setTimeout(resolve, 10));

    expect(useDoctor.getState().phase).toBe('checking');
  });
});

describe('choosing a problem', () => {
  async function opened(): Promise<void> {
    serve(doctor(jsonResponse(report())));
    openDoctor();
    await vi.waitFor(() => {
      expect(useDoctor.getState().phase).toBe('done');
    });
  }

  it('moves through the problems and stays inside the list', async () => {
    await opened();

    moveProblem(-1);
    expect(useDoctor.getState().cursor).toBe(0);
    moveProblem(1);
    moveProblem(1);
    expect(useDoctor.getState().cursor).toBe(1);
    chooseProblem(0);
    expect(useDoctor.getState().cursor).toBe(0);
    chooseProblem(7);
    expect(useDoctor.getState().cursor).toBe(1);
  });

  it('goes to the track of the problem: the dialog closes and the tree shows the track', async () => {
    await opened();
    chooseProblem(1);
    const before = libraryReloads();

    goToProblem();

    expect(useApp.getState().overlay).toBeNull();
    await vi.waitFor(() => {
      expect(useApp.getState().cursor).toBe('track\u001fsowhat');
    });
    expect(libraryReloads()).toBe(before + 1);
  });

  it('has nowhere to go while it is still checking, or when there are no problems', async () => {
    serve(doctor(jsonResponse({ problems: [], errors: 0, warnings: 0 })));
    openDoctor();
    goToProblem();
    expect(useApp.getState().overlay).toEqual({ kind: 'doctor' });
    await vi.waitFor(() => {
      expect(useDoctor.getState().phase).toBe('done');
    });

    moveProblem(1);
    goToProblem();

    expect(useApp.getState().overlay).toEqual({ kind: 'doctor' });
    expect(useDoctor.getState().cursor).toBe(0);
  });
});

describe('a report that was already on its way when the dialog closed', () => {
  it('is dropped, even when it is read after the dialog closed', async () => {
    const slow = slowAnswer();
    serve(doctor(slow.response));
    openDoctor();
    await vi.waitFor(() => {
      expect(fetchMock).toHaveBeenCalled();
    });

    useApp.setState({ overlay: null });
    slow.finish(report());
    await new Promise((resolve) => setTimeout(resolve, 10));

    expect(useDoctor.getState().phase).toBe('checking');
  });
});
