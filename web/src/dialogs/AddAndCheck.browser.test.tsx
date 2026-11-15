import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { page, userEvent } from 'vitest/browser';
import { render } from 'vitest-browser-react';
import type { AddForm, DraftView, JobView } from '../api/types.gen';
import { App } from '../App';
import { goToRow } from '../state/edit';
import { useAdd } from '../state/add';
import { useDoctor } from '../state/doctor';
import { jobEvent } from '../state/jobs';
import { useApp } from '../state/store';
import { changeResponse, FakeEventSource, infoView, jsonResponse, requestsTo, sampleLibrary, stubServer, type StubRoute } from '../testing/fakes';

const sep = '\u001f';
const soWhat = `track${sep}sowhat`;

let fetchMock: ReturnType<typeof stubServer>;

function dialog() {
  return page.getByRole('dialog');
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

describe('the add dialog', () => {
  const offered: AddForm = { title: 'So What', artist: 'Miles Davis', album: 'Kind of Blue', track: '1', of: '5', released: '1959-08-17', genre: '', cutStart: '', cutEnd: '' };

  function draft(over: Partial<DraftView> = {}): DraftView {
    return { draft: 'd1', form: offered, note: 'From YouTube Music metadata: change anything before adding.', cover: 'c1', ...over };
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
    await expect.element(dialog().getByRole('button', { name: 'Remove cover' })).toHaveFocus();
    await userEvent.keyboard('{ArrowUp}{ArrowUp}');
    await expect.element(field('Cut end')).toHaveFocus();
    await userEvent.keyboard('{Tab}');
    await expect.element(dialog().getByRole('button', { name: 'Replace cover…' })).toHaveFocus();
    await userEvent.keyboard('{Tab}');
    await expect.element(dialog().getByRole('button', { name: 'Remove cover' })).toHaveFocus();
    await userEvent.keyboard('{Tab}');
    await expect.element(field('Title')).toHaveFocus();
  });

  test('a picture chosen replaces the cover and shows at once', async () => {
    await toForm(undefined, (request) =>
      request.method === 'PUT' && request.url.pathname === '/api/add/d1/cover' ? jsonResponse({ cover: 'c2' }) : plain(request),
    );
    const chooser = dialog().getByLabelText('Cover picture').element() as HTMLInputElement;
    const transfer = new DataTransfer();
    transfer.items.add(new File([new Uint8Array([1, 2, 3])], 'cover.png', { type: 'image/png' }));

    chooser.files = transfer.files;
    chooser.dispatchEvent(new Event('change', { bubbles: true }));

    await expect.poll(() => dialog().getByRole('img', { name: 'Cover' }).element().getAttribute('src')).toBe('/api/add/d1/cover?v=c2');
  });

  test('a picture the server refuses is said under the cover and the old one stays', async () => {
    await toForm(undefined, (request) =>
      request.method === 'PUT' && request.url.pathname === '/api/add/d1/cover' ? jsonResponse({ error: 'A cover must be a JPEG or PNG image' }, 415) : plain(request),
    );
    const chooser = dialog().getByLabelText('Cover picture').element() as HTMLInputElement;
    const transfer = new DataTransfer();
    transfer.items.add(new File(['text'], 'a.txt', { type: 'text/plain' }));

    chooser.files = transfer.files;
    chooser.dispatchEvent(new Event('change', { bubbles: true }));

    await expect.element(dialog().getByRole('alert')).toHaveTextContent('A cover must be a JPEG or PNG image');
    expect(dialog().getByRole('img', { name: 'Cover' }).element().getAttribute('src')).toBe('/api/add/d1/cover?v=c1');
  });

  test('enter on a button of the cover presses it and does not add the track', async () => {
    await toForm(undefined, (request) =>
      request.method === 'DELETE' && request.url.pathname === '/api/add/d1/cover' ? jsonResponse({ cover: '' }) : plain(request),
    );
    dialog().getByRole('button', { name: 'Remove cover' }).element().focus();

    await userEvent.keyboard('{Enter}');

    await expect.element(dialog().getByRole('button', { name: 'Add a cover…' })).toBeVisible();
    expect(requestsTo(fetchMock, 'POST', '/api/add/finish')).toEqual([]);
  });

});
