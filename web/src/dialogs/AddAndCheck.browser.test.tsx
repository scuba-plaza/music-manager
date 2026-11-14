import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { page, userEvent } from 'vitest/browser';
import { render } from 'vitest-browser-react';
import type { AddForm, DoctorView, DraftView, JobView } from '../api/types.gen';
import { App } from '../App';
import { goToRow } from '../state/edit';
import { useAdd } from '../state/add';
import { useDoctor } from '../state/doctor';
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
  useDoctor.setState(useDoctor.getInitialState(), true);
  useAdd.setState(useAdd.getInitialState(), true);
  FakeEventSource.instances = [];
});

afterEach(async () => {
  useApp.setState({ overlay: null });
  jobEvent({ id: 'a1', kind: 'add', state: 'stopped' });
  await new Promise((done) => setTimeout(done, 80));
  vi.useRealTimers();
  vi.unstubAllGlobals();
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
    await userEvent.keyboard('{Shift>}v{/Shift}');
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
  const offered: AddForm = { title: 'So What', artist: 'Miles Davis', album: 'Kind of Blue', track: '1', of: '5', released: '1959-08-17', genre: '', cutStart: '', cutEnd: '' };
  const names = ['Title', 'Artist', 'Album', 'Track', 'Tracks on album', 'Released', 'Genre', 'Cut start', 'Cut end'];

  function draft(over: Partial<DraftView> = {}): DraftView {
    return { draft: 'd1', form: offered, ...over };
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

  test('the form has a field for each value, in order, the first one focused and filled from what the server offered', async () => {
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
    await expect.element(dialog()).toHaveTextContent('These are YouTube’s words');
    await expect.poll(hints).toEqual(['enter add', 'tab next field', 'esc close']);
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

    await expect.poll(() => requestsTo(fetchMock, 'POST', '/api/add/finish')).toEqual([{ draft: 'd1', form: { ...offered, genre: 'Jazz' } }]);
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
