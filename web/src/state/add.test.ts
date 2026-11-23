import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { AddForm, AlbumView, DraftView, DuplicateView, Fetched, JobView } from '../api/types.gen';
import { changeResponse, infoView, jsonResponse, memoryStorage, requestsTo, sampleLibrary, stubServer, type StubRoute , urlOf } from '../testing/fakes';
import { chooseDraftCover, chooseFiles, fetchLink, leaveField, openAdd, pickAll, removeDraftCover, saveAdd, saveAlbum, setAlbumField, setField, togglePick, showDuplicate, typeLink, useAdd } from './add';
import { shownErrors } from './form';
import { jobEvent } from './jobs';
import { useApp } from './store';

let fetchMock: ReturnType<typeof stubServer>;

const blank: AddForm = { title: '', artist: '', album: '', track: '', of: '', released: '', genre: '', cutStart: '', cutEnd: '' };
const offered: AddForm = { title: 'So What', artist: 'Miles Davis', album: 'Kind of Blue', track: '1', of: '5', released: '1959-08-17', genre: '', cutStart: '', cutEnd: '' };

function draft(over: Partial<DraftView> = {}): DraftView {
  return { draft: 'd1', form: offered, note: 'From YouTube Music metadata: change anything before adding.', cover: 'c1', ...over };
}

function job(state: string, data?: Fetched, extra: Partial<JobView> = {}): JobView {
  return { id: 'a1', kind: 'add', state, ...(data === undefined ? {} : { data }), ...extra };
}

function serve(route: StubRoute = () => undefined, canAdd = true): void {
  fetchMock = stubServer(infoView({ canAdd }), sampleLibrary(), { route });
  useApp.setState({ lib: sampleLibrary(), info: infoView({ canAdd }) });
}

const started: StubRoute = ({ method, url }) => (method === 'POST' && url.pathname === '/api/add/fetch' ? jsonResponse(job('running')) : undefined);

const path: StubRoute = ({ method, url, body }) => {
  if (method === 'POST' && url.pathname === '/api/add/path') {
    const form = (body as { form: AddForm }).form;
    return jsonResponse({ path: `${form.artist}/${form.album}/${form.title}.mp3` });
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

function state() {
  return useAdd.getState();
}

async function fetched(data: Fetched, extra: StubRoute = () => undefined): Promise<void> {
  serve(routes(started, extra, path));
  openAdd();
  typeLink(' https://www.youtube.com/watch?v=abc ');
  fetchLink();
  await vi.waitFor(() => {
    expect(state().job?.id).toBe('a1');
  });
  jobEvent(job('done', data));
}

async function inForm(): Promise<void> {
  await fetched({ draft: draft() });
  await vi.waitFor(() => {
    expect(state().phase).toBe('form');
  });
}

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'setInterval', 'clearInterval'] });
  vi.stubGlobal('localStorage', memoryStorage());
  useApp.setState(useApp.getInitialState(), true);
  useAdd.setState(useAdd.getInitialState(), true);
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe('a', () => {
  it('says when yt-dlp is not set up, and opens nothing', () => {
    serve(() => undefined, false);

    openAdd();

    expect(useApp.getState().overlay).toBeNull();
    expect(useApp.getState().status).toEqual({ text: 'Adding from YouTube is not available: yt-dlp is not set up', kind: 'bad' });
  });

  it('opens the dialog asking for a link, starting again from nothing each time', async () => {
    await inForm();
    useApp.setState({ overlay: null });

    openAdd();

    expect(useApp.getState().overlay).toEqual({ kind: 'add' });
    expect(state()).toMatchObject({ phase: 'asking', link: '', error: '', linkError: '', job: null, draft: null, form: blank, pathNote: '' });
  });

  it('asks for a link first when there is none, until something is typed', () => {
    serve();
    openAdd();
    typeLink('   ');

    fetchLink();

    expect(state()).toMatchObject({ phase: 'asking', linkError: 'Paste a link first' });
    expect(fetchMock).not.toHaveBeenCalled();

    typeLink('h');

    expect(state().linkError).toBe('');
  });
});

describe('fetching a link', () => {
  it('sends the link without the spaces around it and waits for the download', async () => {
    serve(started);
    openAdd();
    typeLink('  https://www.youtube.com/watch?v=abc ');

    fetchLink();

    expect(state()).toMatchObject({ phase: 'fetching', link: 'https://www.youtube.com/watch?v=abc' });
    await vi.waitFor(() => {
      expect(state().job).toEqual(job('running'));
    });
    expect(requestsTo(fetchMock, 'POST', '/api/add/fetch')).toEqual([{ url: 'https://www.youtube.com/watch?v=abc' }]);
  });

  it('fills the form from what the server offered, and asks where the file would go', async () => {
    await fetched({ draft: draft() });
    await vi.waitFor(() => {
      expect(state().phase).toBe('form');
    });

    expect(state()).toMatchObject({ form: offered, draft: draft() });
    await vi.advanceTimersByTimeAsync(120);
    await vi.waitFor(() => {
      expect(state().pathNote).toBe('Miles Davis/Kind of Blue/So What.mp3');
    });
    expect(requestsTo(fetchMock, 'POST', '/api/add/path')).toEqual([{ form: offered }]);
  });

  it('flags a track that is in the library already, and can show it', async () => {
    const existing: DuplicateView = { id: 'sowhat', title: 'So What', artist: 'Miles Davis' };
    await fetched({ duplicate: existing });
    await vi.waitFor(() => {
      expect(state().phase).toBe('duplicate');
    });
    expect(state().existing).toEqual(existing);
    const reloads = () => fetchMock.mock.calls.filter(([input]) => urlOf(input).startsWith('/api/library')).length;
    const before = reloads();

    showDuplicate();

    expect(useApp.getState().overlay).toBeNull();
    await vi.waitFor(() => {
      expect(useApp.getState().cursor).toBe('track\u001fsowhat');
    });
    expect(reloads()).toBe(before + 1);
  });

  it('says why the download failed, and tries again on request', async () => {
    serve(routes(started, ({ method, url }) => (method === 'POST' && url.pathname === '/api/add/path' ? jsonResponse({ path: '' }) : undefined)));
    openAdd();
    typeLink('https://example.com/x');
    fetchLink();
    await vi.waitFor(() => {
      expect(state().job?.id).toBe('a1');
    });

    jobEvent(job('failed', undefined, { error: 'download https://example.com/x: no video here' }));

    await vi.waitFor(() => {
      expect(state()).toMatchObject({ phase: 'failed', error: 'download https://example.com/x: no video here' });
    });
    fetchLink();
    expect(state().phase).toBe('fetching');
    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'POST', '/api/add/fetch')).toHaveLength(2);
    });
  });

  it('says on the link what is wrong with it when the server will not take it, and keeps asking', async () => {
    serve(({ method, url }) =>
      method === 'POST' && url.pathname === '/api/add/fetch'
        ? jsonResponse({ error: 'That is not a web link', fields: { link: 'That is not a web link: it should start with http:// or https://' } }, 422)
        : undefined,
    );
    openAdd();
    typeLink('youtube.com/watch?v=1');

    fetchLink();

    await vi.waitFor(() => {
      expect(state().linkError).toBe('That is not a web link: it should start with http:// or https://');
    });
    expect(state()).toMatchObject({ phase: 'asking', error: '' });
  });

  it('says why the server would not start the download', async () => {
    serve(({ method, url }) => (method === 'POST' && url.pathname === '/api/add/fetch' ? jsonResponse({ error: 'a download is going on already' }, 409) : undefined));
    openAdd();
    typeLink('https://example.com/x');

    fetchLink();

    await vi.waitFor(() => {
      expect(state()).toMatchObject({ phase: 'failed', error: 'a download is going on already' });
    });
  });

  it('ignores a download that was stopped', async () => {
    serve(started);
    openAdd();
    typeLink('https://example.com/x');
    fetchLink();
    await vi.waitFor(() => {
      expect(state().job?.id).toBe('a1');
    });

    jobEvent(job('stopped'));
    await vi.advanceTimersByTimeAsync(10);

    expect(state().phase).toBe('fetching');
  });

  it('ignores a download that ends after the dialog was closed', async () => {
    serve(started);
    openAdd();
    typeLink('https://example.com/x');
    fetchLink();
    await vi.waitFor(() => {
      expect(state().job?.id).toBe('a1');
    });

    useApp.setState({ overlay: null });
    jobEvent(job('done', { draft: draft() }));
    await vi.advanceTimersByTimeAsync(10);

    expect(state().phase).toBe('fetching');
    expect(state().draft).toBeNull();
  });

  it('stops the download when the dialog is left while it is on its way', async () => {
    serve(routes(started, ({ method, url }) => (method === 'DELETE' && url.pathname === '/api/jobs/a1' ? jsonResponse(job('stopped')) : undefined)));
    openAdd();
    typeLink('https://example.com/x');
    fetchLink();
    await vi.waitFor(() => {
      expect(state().job?.id).toBe('a1');
    });

    useApp.setState({ overlay: null });

    expect(useApp.getState().overlay).toBeNull();
    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'DELETE', '/api/jobs/a1')).toEqual([null]);
    });
  });
});

describe('the form', () => {
  it('takes what is typed, and asks again where the file would go, once the typing pauses', async () => {
    await inForm();
    await vi.advanceTimersByTimeAsync(120);
    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'POST', '/api/add/path')).toHaveLength(1);
    });

    setField('title', 'So');
    await vi.advanceTimersByTimeAsync(100);
    setField('title', 'So What (take 2)');
    await vi.advanceTimersByTimeAsync(119);
    expect(requestsTo(fetchMock, 'POST', '/api/add/path')).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(1);

    await vi.waitFor(() => {
      expect(state().pathNote).toBe('Miles Davis/Kind of Blue/So What (take 2).mp3');
    });
    expect(requestsTo(fetchMock, 'POST', '/api/add/path').at(-1)).toEqual({ form: { ...offered, title: 'So What (take 2)' } });
    expect(requestsTo(fetchMock, 'POST', '/api/add/path')).toHaveLength(2);
  });

  it('keeps by field what the server found wrong with the form, and shows a message once its field is left', async () => {
    await fetched(
      { draft: draft() },
      ({ method, url }) =>
        method === 'POST' && url.pathname === '/api/add/path' ? jsonResponse({ path: 'x', fields: { released: 'Not a date', track: 'Track must be a whole number, or empty' } }) : undefined,
    );
    await vi.waitFor(() => {
      expect(state().phase).toBe('form');
    });
    await vi.advanceTimersByTimeAsync(120);
    await vi.waitFor(() => {
      expect(state().checks.wrong).toEqual({ released: 'Not a date', track: 'Track must be a whole number, or empty' });
    });
    expect(shownErrors(state().checks)).toEqual({});

    leaveField('released');

    expect(shownErrors(state().checks)).toEqual({ released: 'Not a date' });
  });

  it('shows only the latest answer about the path', async () => {
    const answers: ((res: Response) => void)[] = [];
    await fetched(
      { draft: draft() },
      ({ method, url }) =>
        method === 'POST' && url.pathname === '/api/add/path'
          ? new Promise<Response>((resolve) => {
              answers.push(resolve);
            })
          : undefined,
    );
    await vi.waitFor(() => {
      expect(state().phase).toBe('form');
    });
    await vi.advanceTimersByTimeAsync(120);
    setField('title', 'B');
    await vi.advanceTimersByTimeAsync(120);
    expect(answers).toHaveLength(2);

    answers[1]?.(jsonResponse({ path: 'new' }));
    await vi.waitFor(() => {
      expect(state().pathNote).toBe('new');
    });
    answers[0]?.(jsonResponse({ path: 'old' }));
    await vi.advanceTimersByTimeAsync(10);

    expect(state().pathNote).toBe('new');
  });

  it('shows no path when the server cannot work one out', async () => {
    await fetched({ draft: draft() }, ({ method, url }) => (method === 'POST' && url.pathname === '/api/add/path' ? jsonResponse({ error: 'no' }, 422) : undefined));
    await vi.waitFor(() => {
      expect(state().phase).toBe('form');
    });
    useAdd.setState({ pathNote: 'stale' });

    await vi.advanceTimersByTimeAsync(120);

    await vi.waitFor(() => {
      expect(state().pathNote).toBe('');
    });
  });
});

describe('the cover of the download', () => {
  const putCover = (sha: string): StubRoute => ({ method, url }) =>
    method === 'PUT' && url.pathname === '/api/add/d1/cover' ? jsonResponse({ cover: sha }) : undefined;

  it('is replaced by a picture chosen, and what the server says is the cover', async () => {
    await fetched({ draft: draft() }, putCover('c2'));
    await vi.waitFor(() => {
      expect(state().phase).toBe('form');
    });
    const picture = new File([new Uint8Array([1, 2, 3])], 'cover.png', { type: 'image/png' });

    await chooseDraftCover(picture);

    expect(state().cover).toBe('c2');
    expect(state().coverError).toBe('');
    expect(fetchMock.mock.calls.some(([input, init]) => urlOf(input) === '/api/add/d1/cover' && init?.method === 'PUT' && init.body instanceof File)).toBe(true);
  });

  it('keeps the old cover and says why when the picture is refused', async () => {
    await fetched({ draft: draft() }, ({ method, url }) =>
      method === 'PUT' && url.pathname === '/api/add/d1/cover' ? jsonResponse({ error: 'A cover must be a JPEG or PNG image' }, 415) : undefined,
    );
    await vi.waitFor(() => {
      expect(state().phase).toBe('form');
    });

    await chooseDraftCover(new File(['text'], 'a.txt', { type: 'text/plain' }));

    expect(state()).toMatchObject({ cover: 'c1', coverError: 'A cover must be a JPEG or PNG image' });
  });

  it('is taken away on request', async () => {
    await fetched({ draft: draft() }, ({ method, url }) => (method === 'DELETE' && url.pathname === '/api/add/d1/cover' ? jsonResponse({ cover: '' }) : undefined));
    await vi.waitFor(() => {
      expect(state().phase).toBe('form');
    });

    await removeDraftCover();

    expect(state().cover).toBe('');
    expect(requestsTo(fetchMock, 'DELETE', '/api/add/d1/cover')).toEqual([null]);
  });
});

describe('adding the track', () => {
  const finish: StubRoute = ({ method, url }) =>
    method === 'POST' && url.pathname === '/api/add/finish' ? jsonResponse(changeResponse({ message: 'Added So What · u undoes it', reveal: 'new1' })) : undefined;
  const discard: StubRoute = ({ method, url }) => (method === 'DELETE' && url.pathname === '/api/add/d1' ? new Response(null, { status: 204 }) : undefined);

  async function inFormWith(...extra: StubRoute[]): Promise<void> {
    await fetched({ draft: draft() }, routes(...extra));
    await vi.waitFor(() => {
      expect(state().phase).toBe('form');
    });
  }

  it('sends the draft with the form, then closes the dialog and keeps the download', async () => {
    await inFormWith(finish, discard);
    setField('title', 'So What (take 2)');

    await saveAdd();

    expect(requestsTo(fetchMock, 'POST', '/api/add/finish')).toEqual([{ draft: 'd1', form: { ...offered, title: 'So What (take 2)' } }]);
    expect(useApp.getState().overlay).toBeNull();
    expect(useApp.getState().status.text).toBe('Added So What · u undoes it');
    expect(requestsTo(fetchMock, 'DELETE', '/api/add/d1')).toEqual([]);
  });

  it('stays in the form and says what is wrong when the server will not add it', async () => {
    await inFormWith(({ method, url }) => (method === 'POST' && url.pathname === '/api/add/finish' ? jsonResponse({ error: 'a track needs a title' }, 422) : undefined), discard);

    await saveAdd();

    expect(state()).toMatchObject({ phase: 'form', error: 'a track needs a title' });
    expect(useApp.getState().overlay).toEqual({ kind: 'add' });
    expect(useApp.getState().busy).toBe(false);
  });

  it('shows every field the server found wrong and says which comes first in the form', async () => {
    await inFormWith(
      ({ method, url }) =>
        method === 'POST' && url.pathname === '/api/add/finish'
          ? jsonResponse({ error: '2 fields need fixing', fields: { cutStart: 'Cut start: invalid time "x"', released: 'Not a date' } }, 422)
          : undefined,
      discard,
    );

    const wrong = await saveAdd();

    expect(wrong).toBe('released');
    expect(shownErrors(state().checks)).toEqual({ cutStart: 'Cut start: invalid time "x"', released: 'Not a date' });
    expect(state()).toMatchObject({ phase: 'form', error: '' });
    expect(useApp.getState().overlay).toEqual({ kind: 'add' });
  });

  it('stays in the form without a message when another change is on its way', async () => {
    await inFormWith(finish, discard);
    useApp.setState({ busy: true });

    await saveAdd();

    expect(state()).toMatchObject({ phase: 'form', error: '' });
    expect(requestsTo(fetchMock, 'POST', '/api/add/finish')).toEqual([]);
  });

  it('forgets the download when the dialog is closed without adding it', async () => {
    await inFormWith(finish, discard);

    useApp.setState({ overlay: null });

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'DELETE', '/api/add/d1')).toEqual([null]);
    });
  });

  it('keeps asking nothing after the form has been closed once', async () => {
    await inFormWith(finish, discard);
    useApp.setState({ overlay: null });
    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'DELETE', '/api/add/d1')).toHaveLength(1);
    });

    useApp.setState({ overlay: { kind: 'help' } });
    useApp.setState({ overlay: null });

    expect(requestsTo(fetchMock, 'DELETE', '/api/add/d1')).toHaveLength(1);
  });
});

describe('MP3 files chosen instead of a link', () => {
  it('closes the dialog and adds them', async () => {
    serve(({ method, url }) => (method === 'POST' && url.pathname === '/api/ingest' ? jsonResponse(changeResponse({ message: 'Added 1 track' })) : undefined));
    openAdd();

    chooseFiles([new File(['ID3'], 'one.mp3', { type: 'audio/mpeg' })]);

    expect(useApp.getState().overlay).toBeNull();
    await vi.waitFor(() => {
      expect(useApp.getState().status.text).toBe('Added 1 track');
    });
  });

  it('does nothing when no file was chosen', () => {
    serve();
    openAdd();

    chooseFiles([]);

    expect(useApp.getState().overlay).toEqual({ kind: 'add' });
  });
});

describe('what was left over from an earlier dialog', () => {
  it('a download that ends after the dialog was opened again changes nothing', async () => {
    serve(started);
    openAdd();
    typeLink('https://example.com/x');
    fetchLink();
    await vi.waitFor(() => {
      expect(state().job?.id).toBe('a1');
    });
    useApp.setState({ overlay: null });
    openAdd();

    jobEvent(job('done', { draft: draft() }));
    await vi.advanceTimersByTimeAsync(10);

    expect(state()).toMatchObject({ phase: 'asking', draft: null });
  });

  it('an answer about a path that arrives after the dialog was opened again changes nothing', async () => {
    let answerLate: (res: Response) => void = () => undefined;
    await fetched(
      { draft: draft() },
      ({ method, url }) =>
        method === 'POST' && url.pathname === '/api/add/path'
          ? new Promise<Response>((resolve) => {
              answerLate = resolve;
            })
          : undefined,
    );
    await vi.waitFor(() => {
      expect(state().phase).toBe('form');
    });
    await vi.advanceTimersByTimeAsync(120);
    expect(requestsTo(fetchMock, 'POST', '/api/add/path')).toHaveLength(1);
    useApp.setState({ overlay: null });
    openAdd();

    answerLate(jsonResponse({ path: 'old' }));
    await vi.advanceTimersByTimeAsync(10);

    expect(state().pathNote).toBe('');
  });

  it('typing that was waiting to ask about a path asks nothing once the dialog is closed', async () => {
    await inForm();
    await vi.advanceTimersByTimeAsync(120);
    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'POST', '/api/add/path')).toHaveLength(1);
    });
    setField('title', 'So');

    useApp.setState({ overlay: null });
    await vi.advanceTimersByTimeAsync(300);

    expect(requestsTo(fetchMock, 'POST', '/api/add/path')).toHaveLength(1);
  });
});

describe('with nothing to act on', () => {
  it('adding does nothing before a download has come to a draft', async () => {
    serve(routes(started, ({ method, url }) => (method === 'POST' && url.pathname === '/api/add/finish' ? jsonResponse(changeResponse({ message: 'x' })) : undefined)));
    openAdd();

    await saveAdd();

    expect(requestsTo(fetchMock, 'POST', '/api/add/finish')).toEqual([]);
    expect(state().phase).toBe('asking');
  });

  it('showing the duplicate does nothing before a duplicate was found', () => {
    serve();
    openAdd();

    showDuplicate();

    expect(useApp.getState().overlay).toEqual({ kind: 'add' });
  });
});

const sampleAlbum: AlbumView = {
  id: 'al1',
  form: { album: 'Kind of Blue', artist: 'Miles Davis', released: '1959', genre: 'Jazz' },
  note: 'From a playlist',
  cover: 'c9',
  tracks: [
    { number: 1, title: 'So What', seconds: 545 },
    { number: 2, title: 'Freddie Freeloader', seconds: 589 },
    { number: 3, title: 'Blue in Green', seconds: 337 },
  ],
};

describe('an album review', () => {
  const addAlbum = (answer: () => Response): StubRoute => ({ method, url }) =>
    method === 'POST' && url.pathname === '/api/add/album' ? answer() : undefined;
  const running = addAlbum(() => jsonResponse({ id: 'b1', kind: 'addalbum', state: 'running' }));
  const discardAlbum: StubRoute = ({ method, url }) => (method === 'DELETE' && url.pathname === '/api/add/al1' ? new Response(null, { status: 204 }) : undefined);
  const stopB1: StubRoute = ({ method, url }) => (method === 'DELETE' && url.pathname === '/api/jobs/b1' ? jsonResponse({ id: 'b1', kind: 'addalbum', state: 'stopped' }) : undefined);

  async function inAlbum(...extra: StubRoute[]): Promise<void> {
    await fetched({ album: sampleAlbum }, routes(...extra));
    await vi.waitFor(() => {
      expect(state().phase).toBe('album');
    });
  }

  it('receiving an album fills the review and picks every track', async () => {
    await inAlbum();

    expect(state()).toMatchObject({ album: sampleAlbum, albumForm: sampleAlbum.form, picked: [1, 2, 3], cover: 'c9' });
  });

  it('togglePick unticks and reticks, keeping the picks in order', async () => {
    await inAlbum();

    togglePick(2);
    expect(state().picked).toEqual([1, 3]);
    togglePick(2);
    expect(state().picked).toEqual([1, 2, 3]);
  });

  it('pickAll(false) then pickAll(true)', async () => {
    await inAlbum();

    pickAll(false);
    expect(state().picked).toEqual([]);
    pickAll(true);
    expect(state().picked).toEqual([1, 2, 3]);
  });

  it('typing a field clears that field error only', async () => {
    await inAlbum();
    useAdd.setState({ albumErrors: { album: 'x', artist: 'y' } });

    setAlbumField('album', 'Other');

    expect(state().albumForm.album).toBe('Other');
    expect(state().albumErrors).toEqual({ artist: 'y' });
  });

  it('saving with nothing picked says so and sends nothing', async () => {
    await inAlbum(running);
    pickAll(false);

    await saveAlbum();

    expect(state()).toMatchObject({ phase: 'album', error: 'Pick at least one track' });
    expect(requestsTo(fetchMock, 'POST', '/api/add/album')).toEqual([]);
  });

  it('saving sends the id, the form and only the picked numbers in ascending order', async () => {
    await inAlbum(running);
    pickAll(false);
    togglePick(3);
    togglePick(1);
    setAlbumField('genre', 'Cool');

    void saveAlbum();

    await vi.waitFor(() => {
      expect(state().phase).toBe('adding');
      expect(state().job?.id).toBe('b1');
    });
    expect(requestsTo(fetchMock, 'POST', '/api/add/album')).toEqual([{ id: 'al1', form: { ...sampleAlbum.form, genre: 'Cool' }, tracks: [1, 3] }]);
  });

  it('progress from the job shows while adding', async () => {
    await inAlbum(running);
    void saveAlbum();
    await vi.waitFor(() => {
      expect(state().job?.id).toBe('b1');
    });

    jobEvent({ id: 'b1', kind: 'addalbum', state: 'running', data: { done: 1, total: 3, title: 'So What' } });

    expect(state()).toMatchObject({ phase: 'adding', progress: { done: 1, total: 3, title: 'So What' } });
  });

  it('a finished job applies its message and closes the dialog', async () => {
    await inAlbum(running, discardAlbum);
    const saving = saveAlbum();
    await vi.waitFor(() => {
      expect(state().job?.id).toBe('b1');
    });

    jobEvent({ id: 'b1', kind: 'addalbum', state: 'done', data: changeResponse({ message: 'Added 3 tracks', reveal: 'n1' }) });
    await saving;

    expect(useApp.getState().overlay).toBeNull();
    expect(useApp.getState().status.text).toBe('Added 3 tracks');
    expect(requestsTo(fetchMock, 'DELETE', '/api/add/al1')).toEqual([]);
    expect(requestsTo(fetchMock, 'DELETE', '/api/jobs/b1')).toEqual([]);
  });

  it('a failed job shows its error', async () => {
    await inAlbum(running);
    const saving = saveAlbum();
    await vi.waitFor(() => {
      expect(state().job?.id).toBe('b1');
    });

    jobEvent({ id: 'b1', kind: 'addalbum', state: 'failed', error: 'disk full' });
    await saving;

    expect(state()).toMatchObject({ phase: 'failed', error: 'disk full' });
  });

  it('a field error returns to the review and names the field', async () => {
    await inAlbum(addAlbum(() => jsonResponse({ error: '1 field needs fixing', fields: { released: 'Not a date' } }, 422)));

    const wrong = await saveAlbum();

    expect(wrong).toBe('released');
    expect(state()).toMatchObject({ phase: 'album', albumErrors: { released: 'Not a date' } });
  });

  it('closing while adding stops the job', async () => {
    await inAlbum(running, stopB1, discardAlbum);
    void saveAlbum();
    await vi.waitFor(() => {
      expect(state().job?.id).toBe('b1');
    });

    useApp.setState({ overlay: null });

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'DELETE', '/api/jobs/b1')).toEqual([null]);
    });
    expect(requestsTo(fetchMock, 'DELETE', '/api/add/al1')).toEqual([]);
  });

  it('closing the review discards the album on the server', async () => {
    await inAlbum(discardAlbum);

    useApp.setState({ overlay: null });

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'DELETE', '/api/add/al1')).toEqual([null]);
    });
  });
});
