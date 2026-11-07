import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { page, userEvent } from 'vitest/browser';
import { render } from 'vitest-browser-react';
import type { AddForm, CandidateView, DoctorView, DraftView, JobView, SearchView } from '../api/types.gen';
import { App } from '../App';
import { goToRow } from '../state/edit';
import { useAdd } from '../state/add';
import { useDoctor } from '../state/doctor';
import { useIdentify } from '../state/identify';
import { useIdentifyAll } from '../state/identifyAll';
import { jobEvent } from '../state/jobs';
import { useApp } from '../state/store';
import { changeResponse, FakeEventSource, infoView, jsonResponse, requestsTo, sampleLibrary, stubServer, urlOf, type StubRoute } from '../testing/fakes';

const sep = '\u001f';
const rough = `track${sep}rough`;
const soWhat = `track${sep}sowhat`;

let fetchMock: ReturnType<typeof stubServer>;

function dialog() {
  return page.getByRole('dialog');
}

function hints(): string[] {
  return page
    .getByRole('list', { name: 'Keys' })
    .getByRole('listitem')
    .elements()
    .map((item) => item.textContent.trim());
}

async function openApp(route: StubRoute, info = infoView()): Promise<void> {
  fetchMock = stubServer(info, sampleLibrary(), { route });
  await render(<App />);
  await expect.element(page.getByRole('treeitem').first()).toBeVisible();
}

async function onRow(key: string): Promise<void> {
  goToRow(key);
  page.getByRole('tree').element().focus();
  await expect.element(page.getByRole('tree').getByRole('treeitem', { selected: true })).toBeVisible();
}

function never(): Promise<Response> {
  return new Promise<Response>(() => undefined);
}

function emit(view: JobView): void {
  FakeEventSource.instances[0]?.emit('job', JSON.stringify(view));
}

beforeEach(() => {
  localStorage.clear();
  useApp.setState(useApp.getInitialState(), true);
  useIdentify.setState(useIdentify.getInitialState(), true);
  useIdentifyAll.setState(useIdentifyAll.getInitialState(), true);
  useDoctor.setState(useDoctor.getInitialState(), true);
  useAdd.setState(useAdd.getInitialState(), true);
  FakeEventSource.instances = [];
});

afterEach(async () => {
  useApp.setState({ overlay: null });
  jobEvent({ id: 'a1', kind: 'add', state: 'stopped' });
  jobEvent({ id: 'j1', kind: 'identify-all', state: 'stopped' });
  await new Promise((done) => setTimeout(done, 80));
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe('the identify dialog', () => {
  function candidate(text: string, diff: CandidateView['diff'] = [{ label: 'Title', old: 'Rough Mix', new: 'So What', same: false }]): CandidateView {
    return { text, diff };
  }

  function found(over: Partial<SearchView> = {}): SearchView {
    return {
      search: 's1',
      title: 'Rough Mix',
      artist: 'Basement Tapes',
      candidates: [
        candidate('So What · Miles Davis', [
          { label: 'Title', old: 'Rough Mix', new: 'So What', same: false },
          { label: 'Artist', old: 'Miles Davis', new: 'Miles Davis', same: true },
        ]),
        candidate('وطني · فيروز', [{ label: 'Title', old: 'Rough Mix', new: 'وطني', same: false }]),
        candidate('Blue in Green · Miles Davis'),
      ],
      ...over,
    };
  }

  function serve(answer: Response | Promise<Response>): StubRoute {
    return ({ method, url }) => {
      if (method === 'POST' && url.pathname === '/api/tracks/rough/candidates') {
        return answer;
      }
      if (method === 'POST' && url.pathname === '/api/identify/apply') {
        return jsonResponse(changeResponse({ message: 'Identified 1 track' }));
      }
      return undefined;
    };
  }

  async function open(answer: Response | Promise<Response>): Promise<void> {
    await openApp(serve(answer));
    await onRow(rough);
    await userEvent.keyboard('i');
    await expect.element(dialog()).toHaveAccessibleName('Identify');
  }

  function chosen(): string | null {
    return (
      dialog()
        .getByRole('option', { selected: true })
        .element()
        .querySelector('.label')?.textContent ?? null
    );
  }

  test('while it searches, the dialog quotes the words and says what esc does', async () => {
    await open(never());

    await expect.element(dialog()).toHaveTextContent('Searching MusicBrainz for “Rough Mix – ”…');
    await expect.poll(hints).toEqual(['esc cancel']);

    await userEvent.keyboard('{Escape}');

    await expect.element(dialog()).not.toBeInTheDocument();
  });

  test('it lists the matches with the first chosen and what choosing it would change under the list', async () => {
    await open(jsonResponse(found()));

    await expect.element(dialog().getByRole('listbox', { name: 'Matches' })).toBeVisible();
    expect(dialog().getByRole('option').elements()).toHaveLength(3);
    expect(chosen()).toBe('So What · Miles Davis');
    await expect.element(dialog()).toHaveTextContent('3 matches for “Rough Mix – Basement Tapes”');
    await expect.element(dialog()).toHaveTextContent('This would change:');
    const diff = dialog().element().querySelector('.diff');
    expect(diff?.children).toHaveLength(4);
    expect([...(diff?.children ?? [])].map((cell) => cell.textContent.trim())).toEqual(['Title', 'Rough Mix → So What', 'Artist', 'Miles Davis  (same)']);
    await expect.poll(hints).toEqual(['↑↓ choose', 'enter apply', '/ other words', 'esc close']);
  });

  test('the arrows, j and k, and the ends move the choice and the diff follows it', async () => {
    await open(jsonResponse(found()));

    await userEvent.keyboard('{ArrowDown}');
    expect(chosen()).toBe('وطني · فيروز');
    await expect.poll(() => dialog().element().querySelector('.diff')?.textContent).toContain('Rough Mix → وطني');
    await userEvent.keyboard('j');
    expect(chosen()).toBe('Blue in Green · Miles Davis');
    await userEvent.keyboard('{ArrowDown}');
    expect(chosen()).toBe('Blue in Green · Miles Davis');
    await userEvent.keyboard('k');
    expect(chosen()).toBe('وطني · فيروز');
    await userEvent.keyboard('{ArrowUp}');
    expect(chosen()).toBe('So What · Miles Davis');
    await userEvent.keyboard('{End}');
    expect(chosen()).toBe('Blue in Green · Miles Davis');
    await userEvent.keyboard('{Home}');
    expect(chosen()).toBe('So What · Miles Davis');
    await userEvent.keyboard('{PageDown}');
    expect(chosen()).toBe('Blue in Green · Miles Davis');
    await userEvent.keyboard('{PageUp}');
    expect(chosen()).toBe('So What · Miles Davis');
  });

  test('a click chooses a match without applying it, and enter applies the chosen one', async () => {
    await open(jsonResponse(found()));

    await dialog().getByRole('option').nth(1).click();

    expect(chosen()).toBe('وطني · فيروز');
    expect(requestsTo(fetchMock, 'POST', '/api/identify/apply')).toEqual([]);

    await userEvent.keyboard('{Enter}');

    await expect.poll(() => requestsTo(fetchMock, 'POST', '/api/identify/apply')).toEqual([{ search: 's1', index: 1 }]);
    await expect.element(dialog()).not.toBeInTheDocument();
  });

  test('q closes the list', async () => {
    await open(jsonResponse(found()));

    await userEvent.keyboard('q');

    await expect.element(dialog()).not.toBeInTheDocument();
    expect(requestsTo(fetchMock, 'POST', '/api/identify/apply')).toEqual([]);
  });

  test('names, matches and changes are isolated for right-to-left text', async () => {
    await open(jsonResponse(found()));

    for (const option of dialog().getByRole('option').elements()) {
      expect(option.querySelector('.label bdi')?.getAttribute('dir')).toBe('auto');
    }
    const diff = dialog().element().querySelector('.diff');
    expect([...(diff?.querySelectorAll('bdi') ?? [])].map((b) => b.getAttribute('dir'))).toEqual(['auto', 'auto', 'auto']);
  });

  test('/ opens the two words to search with, filled in and starting on the title, and enter searches again', async () => {
    const again = jsonResponse(found({ search: 's2', title: 'Blue in Green', artist: 'Miles Davis', candidates: [candidate('Blue in Green · Miles Davis')] }));
    await open(jsonResponse(found()));
    fetchMock.mockClear();

    await userEvent.keyboard('/');

    const title = dialog().getByRole('textbox', { name: 'title' });
    const artist = dialog().getByRole('textbox', { name: 'artist' });
    await expect.element(title).toHaveFocus();
    await expect.element(title).toHaveValue('Rough Mix');
    await expect.element(artist).toHaveValue('Basement Tapes');
    await expect.poll(hints).toEqual(['enter search', 'tab next', 'esc back']);

    await userEvent.keyboard('{ArrowDown}');
    await expect.element(artist).toHaveFocus();
    await userEvent.keyboard('{ArrowUp}');
    await expect.element(title).toHaveFocus();
    await userEvent.fill(title, ' Blue in Green ');
    await userEvent.fill(artist, 'Miles Davis');
    fetchMock.mockImplementation(() => Promise.resolve(again));
    await userEvent.keyboard('{Enter}');

    await expect.poll(() => requestsTo(fetchMock, 'POST', '/api/tracks/rough/candidates')).toEqual([{ query: { title: 'Blue in Green', artist: 'Miles Davis' } }]);
    await expect.element(dialog()).toHaveTextContent('1 match for “Blue in Green – Miles Davis”');
  });

  test('esc in the words goes back to the list instead of closing', async () => {
    await open(jsonResponse(found()));
    await userEvent.keyboard('/');
    await expect.element(dialog().getByRole('textbox', { name: 'title' })).toBeVisible();

    await userEvent.keyboard('{Escape}');

    await expect.element(dialog().getByRole('listbox', { name: 'Matches' })).toBeVisible();
    await userEvent.keyboard('{Escape}');
    await expect.element(dialog()).not.toBeInTheDocument();
  });

  test('when nothing is found it says so with the words, and / is offered', async () => {
    await open(jsonResponse(found({ candidates: [] })));

    await expect.element(dialog().getByRole('alert')).toHaveTextContent('Nothing found on MusicBrainz for “Rough Mix – Basement Tapes”');
    await expect.element(dialog()).toHaveTextContent('Press / to search with other words.');
    await expect.poll(hints).toEqual(['/ other words', 'esc close']);

    await userEvent.keyboard('/');
    await expect.element(dialog().getByRole('textbox', { name: 'title' })).toHaveFocus();
    await userEvent.keyboard('{Escape}');
    await expect.element(dialog().getByRole('alert')).toBeVisible();

    await userEvent.keyboard('q');
    await expect.element(dialog()).not.toBeInTheDocument();
  });

  test('a failed search shows the server’s words', async () => {
    await open(jsonResponse({ error: 'MusicBrainz is busy' }, 503));

    await expect.element(dialog().getByRole('alert')).toHaveTextContent('MusicBrainz is busy');
  });

  test('while applying it says so, and esc leaves at once and says the cover was skipped', async () => {
    let finish: (res: Response) => void = () => undefined;
    const pending = new Promise<Response>((resolve) => {
      finish = resolve;
    });
    await openApp(({ method, url }) => {
      if (method === 'POST' && url.pathname === '/api/tracks/rough/candidates') {
        return jsonResponse(found());
      }
      return method === 'POST' && url.pathname === '/api/identify/apply' ? pending : undefined;
    });
    await onRow(rough);
    await userEvent.keyboard('i');
    await expect.element(dialog().getByRole('listbox', { name: 'Matches' })).toBeVisible();

    await userEvent.keyboard('{Enter}');

    await expect.element(dialog()).toHaveTextContent('Applying…');
    await expect.element(dialog()).toHaveTextContent('Fetching the cover can take a moment; esc skips it.');
    await expect.poll(hints).toEqual(['esc skip the cover']);
    await userEvent.keyboard('q');
    await expect.element(dialog()).toBeVisible();

    await userEvent.keyboard('{Escape}');

    await expect.element(dialog()).not.toBeInTheDocument();
    await expect.element(page.getByRole('status')).toHaveTextContent('Identified without waiting for the cover');
    finish(jsonResponse(changeResponse({ message: 'late' })));
  });
});

describe('the identify all dialog', () => {
  const progress = (over: Record<string, unknown> = {}) => ({ total: 4, done: 1, identified: 1, left: 0, failed: 0, failures: [], ...over });

  function running(data: unknown = progress()): JobView {
    return { id: 'j1', kind: 'identify-all', state: 'running', data };
  }

  const route: StubRoute = ({ method, url }) => {
    if (method === 'POST' && url.pathname === '/api/identify-all') {
      return jsonResponse(running());
    }
    if (method === 'DELETE' && url.pathname === '/api/jobs/j1') {
      return jsonResponse({ id: 'j1', kind: 'identify-all', state: 'stopped' });
    }
    return undefined;
  };

  async function open(): Promise<void> {
    await openApp(route);
    await onRow(rough);
    await userEvent.keyboard('I');
    await expect.element(dialog()).toHaveTextContent('Identify 1 track on MusicBrainz?');
    await userEvent.keyboard('{Enter}');
    await expect.element(dialog()).toHaveAccessibleName('Identify all');
  }

  function bar() {
    return dialog().getByRole('progressbar', { name: 'Identified so far' });
  }

  test('a bar and the counts follow what the server says', async () => {
    await open();

    await expect.element(dialog()).toHaveTextContent('Identifying tracks on MusicBrainz…');
    await expect.element(bar()).toHaveAttribute('aria-valuemin', '0');
    await expect.element(bar()).toHaveAttribute('aria-valuemax', '4');
    await expect.element(bar()).toHaveAttribute('aria-valuenow', '1');
    expect((bar().element().firstElementChild as HTMLElement).style.inlineSize).toBe('25%');
    await expect.element(dialog()).toHaveTextContent('1/4');
    await expect.element(dialog()).toHaveTextContent('Identified 1 · Left for you 0 · Failed 0');
    await expect.poll(hints).toEqual(['esc stop']);

    emit(running(progress({ done: 3, identified: 2, left: 1 })));

    await expect.element(bar()).toHaveAttribute('aria-valuenow', '3');
    expect((bar().element().firstElementChild as HTMLElement).style.inlineSize).toBe('75%');
    await expect.element(dialog()).toHaveTextContent('Identified 2 · Left for you 1 · Failed 0');
    await expect.element(dialog()).not.toHaveTextContent('closer look');
  });

  test('before the server has said how much there is, the bar is empty', async () => {
    await open();

    emit(running(progress({ total: 0, done: 0, identified: 0 })));

    await expect.element(bar()).toHaveAttribute('aria-valuemax', '0');
    await expect.poll(() => (bar().element().firstElementChild as HTMLElement).style.inlineSize).toBe('0%');
  });

  test('esc while it runs asks the server to stop and keeps the dialog for the result', async () => {
    await open();

    await userEvent.keyboard('{Escape}');

    await expect.poll(() => requestsTo(fetchMock, 'DELETE', '/api/jobs/j1')).toEqual([null]);
    emit({ id: 'j1', kind: 'identify-all', state: 'stopped', data: progress({ done: 2, identified: 2 }) });
    await expect.element(dialog()).toHaveTextContent('Stopped');
    await expect.poll(hints).toEqual(['enter close']);
  });

  test('when it is done the head says so, the rest is explained, and enter closes', async () => {
    await open();

    emit({ id: 'j1', kind: 'identify-all', state: 'done', data: progress({ total: 4, done: 4, identified: 2, left: 2 }) });

    await expect.element(dialog()).toHaveTextContent('Done');
    await expect.element(dialog()).toHaveTextContent('The rest need a closer look: i searches for one track, o uses its old tags.');
    await userEvent.keyboard('{Enter}');
    await expect.element(dialog()).not.toBeInTheDocument();
  });

  test('nothing is explained when nothing is left', async () => {
    await open();

    emit({ id: 'j1', kind: 'identify-all', state: 'done', data: progress({ done: 4, identified: 4, left: 0 }) });

    await expect.element(dialog()).toHaveTextContent('Done');
    await expect.element(dialog()).not.toHaveTextContent('closer look');
  });

  test('the first three failures are listed and the rest are one line', async () => {
    await open();

    emit({ id: 'j1', kind: 'identify-all', state: 'done', data: progress({ done: 4, failed: 4, failures: ['a: no', 'b: no', 'c: no', 'd: no'] }) });

    await expect.element(dialog()).toHaveTextContent('a: no');
    await expect.element(dialog()).toHaveTextContent('c: no');
    await expect.element(dialog()).not.toHaveTextContent('d: no');
    await expect.element(dialog()).toHaveTextContent('…and more');
  });

  test('exactly three failures have no more line', async () => {
    await open();

    emit({ id: 'j1', kind: 'identify-all', state: 'done', data: progress({ done: 4, failed: 3, failures: ['a: no', 'b: no', 'c: no'] }) });

    await expect.element(dialog()).toHaveTextContent('c: no');
    await expect.element(dialog()).not.toHaveTextContent('…and more');
  });

  test('a job that failed shows why', async () => {
    await open();

    emit({ id: 'j1', kind: 'identify-all', state: 'failed', error: 'MusicBrainz went away', data: progress() });

    await expect.element(dialog()).toHaveTextContent('MusicBrainz went away');
  });
});

describe('the library check', () => {
  function result(over: Partial<DoctorView> = {}): DoctorView {
    return {
      errors: 1,
      warnings: 2,
      problems: [
        { severity: 'error', trackId: 'sowhat', title: 'So What', message: 'the file is missing' },
        { severity: 'warning', trackId: 'rough', title: 'Rough Mix', message: 'not identified yet' },
        { severity: 'warning', trackId: 'watani', title: 'وطني', message: 'no cover' },
      ],
      ...over,
    };
  }

  async function open(answer: Response | Promise<Response>): Promise<void> {
    await openApp(({ method, url }) => (method === 'GET' && url.pathname === '/api/doctor' ? answer : undefined));
    await onRow(rough);
    await userEvent.keyboard('D');
    await expect.element(dialog()).toHaveAccessibleName('Library check');
  }

  function chosen(): string | null {
    return dialog().getByRole('option', { selected: true }).element().querySelector('b')?.textContent ?? null;
  }

  function libraryRequests(): string[] {
    return fetchMock.mock.calls.flatMap(([input]) => (urlOf(input).startsWith('/api/library') ? [urlOf(input)] : []));
  }

  test('while it checks it says so and esc cancels', async () => {
    await open(never());

    await expect.element(dialog()).toHaveTextContent('Checking that every stored file is there and unharmed…');
    await expect.poll(hints).toEqual(['esc close']);
    await userEvent.keyboard('{Escape}');
    await expect.element(dialog()).not.toBeInTheDocument();
  });

  test('it lists the problems with their counts, the first one chosen', async () => {
    await open(jsonResponse(result()));

    await expect.element(dialog()).toHaveTextContent('1 error, 2 warnings');
    const list = dialog().getByRole('listbox', { name: 'Problems' });
    await expect.element(list).toBeVisible();
    const options = dialog().getByRole('option').elements();
    expect(options.map((option) => option.textContent.trim())).toEqual(['✗So What · the file is missing', '!Rough Mix · not identified yet', '!وطني · no cover']);
    expect(options.map((option) => option.querySelector('[aria-label]')?.getAttribute('aria-label'))).toEqual(['error', 'warning', 'warning']);
    expect(options.map((option) => option.querySelector('[aria-label]')?.className)).toEqual(['bad', 'warn', 'warn']);
    expect(chosen()).toBe('So What');
    await expect.poll(hints).toEqual(['↑↓ choose', 'enter go to the track', 'esc close']);
  });

  test('titles and messages are isolated for right-to-left text', async () => {
    await open(jsonResponse(result()));

    for (const option of dialog().getByRole('option').elements()) {
      expect([...option.querySelectorAll('bdi')].map((b) => b.getAttribute('dir'))).toEqual(['auto', 'auto']);
    }
  });

  test('the arrows, j and k move the choice and stop at the ends', async () => {
    await open(jsonResponse(result()));

    await userEvent.keyboard('{ArrowUp}');
    expect(chosen()).toBe('So What');
    await userEvent.keyboard('j');
    expect(chosen()).toBe('Rough Mix');
    await userEvent.keyboard('{ArrowDown}');
    expect(chosen()).toBe('وطني');
    await userEvent.keyboard('{ArrowDown}');
    expect(chosen()).toBe('وطني');
    await userEvent.keyboard('k');
    expect(chosen()).toBe('Rough Mix');
    await userEvent.keyboard('{ArrowUp}');
    expect(chosen()).toBe('So What');
  });

  test('enter on a problem closes the check and moves the cursor to that track', async () => {
    await open(jsonResponse(result()));
    await userEvent.keyboard('{ArrowDown}');
    const before = libraryRequests().length;

    await userEvent.keyboard('{Enter}');

    await expect.element(dialog()).not.toBeInTheDocument();
    await expect.poll(() => libraryRequests().length).toBeGreaterThan(before);
    await expect.poll(() => useApp.getState().cursor).toBe(rough);
  });

  test('a click on a problem goes to its track in one step', async () => {
    await open(jsonResponse(result()));

    await dialog().getByRole('option').nth(0).click();

    await expect.element(dialog()).not.toBeInTheDocument();
    await expect.poll(() => useApp.getState().cursor).toBe(soWhat);
  });

  test('with no problems it says so, and keys that would choose do nothing', async () => {
    await open(jsonResponse(result({ errors: 0, warnings: 0, problems: [] })));

    await expect.element(dialog()).toHaveTextContent('No problems found.');
    expect(dialog().getByRole('option').elements()).toHaveLength(0);
    await expect.poll(hints).toEqual(['esc close']);
    await userEvent.keyboard('{Enter}');
    await expect.element(dialog()).toBeVisible();
    await userEvent.keyboard('q');
    await expect.element(dialog()).not.toBeInTheDocument();
  });

  test('a check that fails shows the server’s words', async () => {
    await open(jsonResponse({ error: 'the library is locked' }, 500));

    await expect.element(dialog().getByRole('alert')).toHaveTextContent('the library is locked');
  });
});

describe('the add dialog', () => {
  const matched: AddForm = { title: 'So What', artist: 'Miles Davis', album: 'Kind of Blue', track: '1', of: '5', released: '1959-08-17', genre: '', cutStart: '', cutEnd: '' };
  const downloaded: AddForm = { title: 'so what (official)', artist: 'MilesDavisVEVO', album: '', track: '', of: '', released: '', genre: '', cutStart: '', cutEnd: '' };
  const names = ['Title', 'Artist', 'Album', 'Track', 'Tracks on album', 'Released', 'Genre', 'Cut start', 'Cut end'];

  function draft(over: Partial<DraftView> = {}): DraftView {
    return { draft: 'd1', match: { artist: 'Miles Davis', title: 'So What', album: 'Kind of Blue' }, matched, downloaded, ...over };
  }

  function job(state: string, data?: unknown, error?: string): JobView {
    return { id: 'a1', kind: 'add', state, ...(data === undefined ? {} : { data }), ...(error === undefined ? {} : { error }) };
  }

  const plain: StubRoute = ({ method, url, body }) => {
    if (method === 'POST' && url.pathname === '/api/add/fetch') {
      return jsonResponse(job('running'));
    }
    if (method === 'POST' && url.pathname === '/api/add/path') {
      const form = (body as { form: AddForm }).form;
      return jsonResponse({ path: `${form.artist}/${form.album}/${form.title}.mp3` });
    }
    if (method === 'POST' && url.pathname === '/api/add/finish') {
      return jsonResponse(changeResponse({ message: 'Added So What' }));
    }
    return undefined;
  };

  async function open(route: StubRoute = plain): Promise<void> {
    await openApp(route);
    await onRow(soWhat);
    await userEvent.keyboard('a');
    await expect.element(dialog()).toHaveAccessibleName('Add from YouTube');
  }

  async function fetched(data: unknown, route: StubRoute = plain): Promise<void> {
    await open(route);
    await userEvent.keyboard('https://youtu.be/x{Enter}');
    await expect.element(dialog()).toHaveTextContent('Downloading…');
    emit(job('done', data));
  }

  async function toForm(data: unknown = { draft: draft() }, route: StubRoute = plain): Promise<void> {
    await fetched(data, route);
    await expect.element(dialog().getByRole('textbox', { name: 'Title' })).toBeVisible();
  }

  function field(name: string) {
    return dialog().getByRole('textbox', { name, exact: true });
  }

  test('it starts asking for a link, with the link focused', async () => {
    await open();

    await expect.element(field('Link')).toHaveFocus();
    await expect.element(dialog()).toHaveTextContent('Paste a link to a video or a track:');
    await expect.element(dialog()).toHaveTextContent('MP3 files can also be dropped on the page.');
    await expect.poll(hints).toEqual(['enter fetch', 'esc close']);
    await expect.element(field('Link')).toHaveAttribute('placeholder', 'https://www.youtube.com/watch?v=…');
  });

  test('enter on an empty link complains, and typing takes the complaint away', async () => {
    await open();

    await userEvent.keyboard('{Enter}');

    await expect.element(dialog()).toHaveTextContent('Paste a link first');
    expect(requestsTo(fetchMock, 'POST', '/api/add/fetch')).toEqual([]);
    await userEvent.keyboard('h');
    await expect.element(dialog()).not.toHaveTextContent('Paste a link first');
  });

  test('the button opens the file chooser, and the chooser itself is out of the tab order', async () => {
    await open();
    const chooser = dialog().getByLabelText('MP3 files').element() as HTMLInputElement;
    const click = vi.spyOn(chooser, 'click').mockImplementation(() => undefined);

    await dialog().getByRole('button', { name: 'Choose MP3 files instead…' }).click();

    expect(click).toHaveBeenCalledTimes(1);
    expect(chooser.tabIndex).toBe(-1);
    expect(chooser.accept).toBe('.mp3,audio/mpeg');
    expect(chooser.multiple).toBe(true);
  });

  test('while it downloads it counts the seconds, and esc cancels the download', async () => {
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'] });
    await open();
    await userEvent.keyboard('https://youtu.be/x{Enter}');
    await expect.element(dialog()).toHaveTextContent('Downloading… 0s');
    await expect.element(dialog()).toHaveTextContent('This also looks the track up on MusicBrainz. esc cancels.');
    await expect.poll(hints).toEqual(['esc cancel']);

    await vi.advanceTimersByTimeAsync(3000);

    await expect.element(dialog()).toHaveTextContent('Downloading… 3s');
  });

  test('esc while it downloads closes the dialog and stops the download', async () => {
    await open((request) => (request.method === 'DELETE' && request.url.pathname === '/api/jobs/a1' ? jsonResponse(job('stopped')) : plain(request)));
    await userEvent.keyboard('https://youtu.be/x{Enter}');
    await expect.element(dialog()).toHaveTextContent('Downloading…');
    await expect.poll(() => requestsTo(fetchMock, 'POST', '/api/add/fetch')).toEqual([{ url: 'https://youtu.be/x' }]);

    await userEvent.keyboard('{Escape}');

    await expect.element(dialog()).not.toBeInTheDocument();
    await expect.poll(() => requestsTo(fetchMock, 'DELETE', '/api/jobs/a1')).toEqual([null]);
  });

  test('the form has a field for each value, in order, the first one focused and filled from the match', async () => {
    await toForm();

    expect(
      dialog()
        .getByRole('textbox')
        .elements()
        .map((input) => input.getAttribute('aria-label')),
    ).toEqual(names);
    await expect.element(field('Title')).toHaveFocus();
    await expect.element(field('Title')).toHaveValue('So What');
    await expect.element(field('Artist')).toHaveValue('Miles Davis');
    await expect.element(field('Album')).toHaveValue('Kind of Blue');
    await expect.element(field('Track')).toHaveValue('1');
    await expect.element(field('Tracks on album')).toHaveValue('5');
    await expect.element(field('Released')).toHaveValue('1959-08-17');
    await expect.element(dialog()).toHaveTextContent('Matched on MusicBrainz: Miles Davis - So What · Kind of Blue');
    await expect.poll(hints).toEqual(['enter add', 'tab next field', 'F2 ignore the match', 'esc close']);
    for (const name of ['Title', 'Artist', 'Album', 'Released', 'Genre', 'Cut start', 'Cut end']) {
      await expect.element(field(name)).toHaveAttribute('dir', 'auto');
    }
  });

  test('tab, shift-tab and the up and down arrows go round the fields', async () => {
    await toForm();

    await userEvent.keyboard('{Tab}');
    await expect.element(field('Artist')).toHaveFocus();
    await userEvent.keyboard('{ArrowDown}');
    await expect.element(field('Album')).toHaveFocus();
    await userEvent.keyboard('{ArrowDown}');
    await expect.element(field('Track')).toHaveFocus();
    await userEvent.keyboard('{Tab}');
    await expect.element(field('Tracks on album')).toHaveFocus();
    await userEvent.keyboard('{ArrowUp}');
    await expect.element(field('Track')).toHaveFocus();
    await userEvent.keyboard('{Shift>}{Tab}{/Shift}');
    await expect.element(field('Album')).toHaveFocus();
    await userEvent.keyboard('{ArrowUp}{ArrowUp}{ArrowUp}');
    await expect.element(field('Cut end')).toHaveFocus();
    await userEvent.keyboard('{Tab}');
    await expect.element(field('Title')).toHaveFocus();
  });

  test('F2 swaps the match for what the download said and back, keeping what was typed', async () => {
    await toForm();
    await userEvent.fill(field('Artist'), 'Miles');

    await userEvent.keyboard('{F2}');

    await expect.element(dialog()).toHaveTextContent('Ignoring the match: these are YouTube’s words.');
    await expect.element(field('Title')).toHaveValue('so what (official)');
    await expect.element(field('Artist')).toHaveValue('Miles');
    await expect.poll(hints).toEqual(['enter add', 'tab next field', 'F2 use the match', 'esc close']);

    await userEvent.keyboard('{F2}');

    await expect.element(dialog()).toHaveTextContent('Matched on MusicBrainz');
    await expect.element(field('Title')).toHaveValue('So What');
  });

  test('without a match it says so, and F2 does nothing', async () => {
    const { match, ...rest } = draft();
    expect(match).toBeDefined();
    await toForm({ draft: { ...rest, matched: downloaded } });

    await expect.element(dialog()).toHaveTextContent('No match on MusicBrainz: these are YouTube’s words.');
    await expect.poll(hints).toEqual(['enter add', 'tab next field', 'esc close']);
    await userEvent.keyboard('{F2}');
    await expect.element(dialog()).toHaveTextContent('No match on MusicBrainz');
    await expect.element(field('Title')).toHaveValue('so what (official)');
  });

  test('the path it would be written to follows the fields, after the arrow', async () => {
    await toForm();
    const row = () => dialog().element().querySelector('.pathrow');
    await expect.poll(() => row()?.textContent).toBe('→ Miles Davis/Kind of Blue/So What.mp3');
    expect(dialog().element().querySelector('.pathline')?.textContent).toBe('Miles Davis/Kind of Blue/So What.mp3');

    await userEvent.fill(field('Title'), 'So What (take 2)');

    await expect.poll(() => dialog().element().querySelector('.pathline')?.textContent).toBe('Miles Davis/Kind of Blue/So What (take 2).mp3');
  });

  test('until the path is known there is no arrow either', async () => {
    await toForm(undefined, (request) => (request.method === 'POST' && request.url.pathname === '/api/add/path' ? never() : plain(request)));

    await expect.poll(() => requestsTo(fetchMock, 'POST', '/api/add/path').length).toBe(1);
    expect(dialog().element().querySelector('.pathrow')?.textContent).toBe('');
  });

  test('enter adds the track with the fields as they are, and the dialog closes', async () => {
    await toForm();
    await userEvent.fill(field('Genre'), 'Jazz');

    await userEvent.keyboard('{Enter}');

    await expect.poll(() => requestsTo(fetchMock, 'POST', '/api/add/finish')).toEqual([{ draft: 'd1', form: { ...matched, genre: 'Jazz' } }]);
    await expect.element(dialog()).not.toBeInTheDocument();
    expect(requestsTo(fetchMock, 'DELETE', '/api/add/d1')).toEqual([]);
  });

  test('while it adds it says so', async () => {
    await toForm(undefined, (request) => (request.method === 'POST' && request.url.pathname === '/api/add/finish' ? never() : plain(request)));

    await userEvent.keyboard('{Enter}');

    await expect.element(dialog()).toHaveTextContent('Adding to the library…');
    await expect.poll(hints).toEqual(['esc close']);
  });

  test('a refusal brings the form back with the reason and what was typed', async () => {
    await toForm(undefined, (request) => (request.method === 'POST' && request.url.pathname === '/api/add/finish' ? jsonResponse({ error: 'that file already exists' }, 409) : plain(request)));
    await userEvent.fill(field('Genre'), 'Jazz');

    await userEvent.keyboard('{Enter}');

    await expect.element(dialog()).toHaveTextContent('that file already exists');
    await expect.element(field('Genre')).toHaveValue('Jazz');
    await expect.element(field('Genre')).toHaveFocus();
  });

  test('esc on the form forgets the download', async () => {
    await toForm();

    await userEvent.keyboard('{Escape}');

    await expect.element(dialog()).not.toBeInTheDocument();
    await expect.poll(() => requestsTo(fetchMock, 'DELETE', '/api/add/d1')).toEqual([null]);
  });

  test('a duplicate shows what is already there and enter goes to it', async () => {
    await fetched({ duplicate: { id: 'good', title: 'Feeling Good', artist: 'Nina Simone' } });
    await expect.element(dialog()).toHaveTextContent('This audio is already in your library:');
    await expect.element(dialog()).toHaveTextContent('Feeling Good · Nina Simone');
    expect([...dialog().element().querySelectorAll('.ask-head bdi')].map((b) => b.getAttribute('dir'))).toEqual(['auto', 'auto']);
    await expect.poll(hints).toEqual(['enter show it', 'esc close']);

    await userEvent.keyboard('{Enter}');

    await expect.element(dialog()).not.toBeInTheDocument();
    await expect.poll(() => useApp.getState().cursor).toBe(`track${sep}good`);
  });

  test('a failed download shows why, and enter tries the same link again', async () => {
    await open();
    await userEvent.keyboard('https://youtu.be/x{Enter}');
    await expect.element(dialog()).toHaveTextContent('Downloading…');

    emit(job('failed', undefined, 'yt-dlp: video unavailable'));

    await expect.element(dialog().getByRole('alert')).toHaveTextContent('yt-dlp: video unavailable');
    await expect.poll(hints).toEqual(['enter tries again', 'esc close']);
    await userEvent.keyboard('{Enter}');
    await expect.element(dialog()).toHaveTextContent('Downloading…');
    expect(requestsTo(fetchMock, 'POST', '/api/add/fetch')).toEqual([{ url: 'https://youtu.be/x' }, { url: 'https://youtu.be/x' }]);
  });
});
