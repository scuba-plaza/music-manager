import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { page, userEvent } from 'vitest/browser';
import { render } from 'vitest-browser-react';
import type { AlbumView, JobView } from '../api/types.gen';
import { App } from '../App';
import { goToRow } from '../state/edit';
import { useAdd } from '../state/add';
import { useDoctor } from '../state/doctor';
import { jobEvent } from '../state/jobs';
import { useApp } from '../state/store';
import { FakeEventSource, infoView, jsonResponse, requestsTo, sampleLibrary, stubServer, type StubRoute } from '../testing/fakes';

const sep = '\u001f';
const soWhat = `track${sep}sowhat`;

let fetchMock: ReturnType<typeof stubServer>;

const album: AlbumView = {
  id: 'al1',
  form: { album: 'Kind of Blue', artist: 'Miles Davis', released: '1959', genre: '' },
  note: 'From a playlist: change anything before adding.',
  cover: 'abcdef0123456789ffff',
  tracks: [
    { number: 1, title: 'So What', seconds: 545 },
    { number: 2, title: 'مرحبا بالعالم', seconds: 65 },
    { number: 3, title: 'Blue in Green', seconds: 337 },
  ],
};

function job(state: string, data?: unknown): JobView {
  return { id: 'a1', kind: 'add', state, ...(data === undefined ? {} : { data }) };
}

function dialog() {
  return page.getByRole('dialog');
}

function emit(view: JobView): void {
  FakeEventSource.instances[0]?.emit('job', JSON.stringify(view));
}

const routes: StubRoute = ({ method, url }) => {
  if (method === 'POST' && url.pathname === '/api/add/fetch') {
    return jsonResponse(job('running'));
  }
  if (method === 'POST' && url.pathname === '/api/add/album') {
    return jsonResponse({ id: 'b1', kind: 'addalbum', state: 'running' });
  }
  if (method === 'DELETE' && url.pathname === '/api/jobs/b1') {
    return jsonResponse({ id: 'b1', kind: 'addalbum', state: 'stopped' });
  }
  if (method === 'DELETE' && url.pathname === '/api/add/al1') {
    return new Response(null, { status: 204 });
  }
  return undefined;
};

async function inReview(): Promise<void> {
  fetchMock = stubServer(infoView(), sampleLibrary(), { route: routes });
  await render(<App />);
  await expect.element(page.getByRole('treeitem').first()).toBeVisible();
  goToRow(soWhat);
  page.getByRole('tree').element().focus();
  await expect.element(page.getByRole('tree').getByRole('treeitem', { selected: true })).toBeVisible();
  await userEvent.keyboard('a');
  await expect.element(dialog()).toHaveAccessibleName('Add from YouTube');
  await userEvent.keyboard('https://youtube.com/playlist?list=x{Enter}');
  await expect.element(dialog()).toHaveTextContent('Downloading…');
  emit(job('done', { album }));
  await expect.element(dialog().getByRole('textbox', { name: 'Album' })).toBeVisible();
}

function track(name: string) {
  return dialog().getByRole('checkbox', { name });
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

describe('the album review', () => {
  test('the review lists every track ticked with the album details filled in', async () => {
    await inReview();

    await expect.element(dialog()).toHaveTextContent('From a playlist: change anything before adding.');
    await expect.element(dialog().getByRole('textbox', { name: 'Album', exact: true })).toHaveValue('Kind of Blue');
    await expect.element(dialog().getByRole('textbox', { name: 'Artist', exact: true })).toHaveValue('Miles Davis');
    await expect.element(dialog().getByRole('textbox', { name: 'Released', exact: true })).toHaveValue('1959');
    await expect.element(dialog().getByRole('textbox', { name: 'Genre', exact: true })).toHaveAttribute('placeholder', "each track's own");
    await expect.element(dialog().getByRole('textbox', { name: 'Released', exact: true })).toHaveAttribute('placeholder', "each track's own");
    await expect.element(dialog().getByRole('img', { name: 'Cover' })).toHaveAttribute('src', '/api/add/al1/cover?v=abcdef0123456789');
    await expect.element(dialog()).toHaveTextContent('3 of 3 selected');
    await expect.element(track('1. So What')).toBeChecked();
    await expect.element(track('3. Blue in Green')).toBeChecked();
    await expect.element(dialog()).toHaveTextContent('9:05');
    await expect.element(dialog()).toHaveTextContent('1:05');
    await expect.element(dialog().getByRole('button', { name: 'Select all' })).toBeVisible();
    await expect.element(dialog().getByRole('button', { name: 'Select none' })).toBeVisible();
  });

  test('an Arabic title is shown in a bdi with dir auto', async () => {
    await inReview();

    const bdi = dialog().getByText('مرحبا بالعالم').element();
    expect(bdi.tagName).toBe('BDI');
    expect(bdi.getAttribute('dir')).toBe('auto');
  });

  test('space unticks the focused track and the count follows', async () => {
    await inReview();

    track('2. مرحبا بالعالم').element().focus();
    await userEvent.keyboard(' ');

    await expect.element(track('2. مرحبا بالعالم')).not.toBeChecked();
    await expect.element(dialog()).toHaveTextContent('2 of 3 selected');
  });

  test('Select none then Enter says to pick a track and sends nothing', async () => {
    await inReview();

    await dialog().getByRole('button', { name: 'Select none' }).click();
    await expect.element(dialog()).toHaveTextContent('0 of 3 selected');
    dialog().getByRole('textbox', { name: 'Album', exact: true }).element().focus();
    await userEvent.keyboard('{Enter}');

    await expect.element(dialog()).toHaveTextContent('Pick at least one track');
    expect(requestsTo(fetchMock, 'POST', '/api/add/album')).toEqual([]);

    await dialog().getByRole('button', { name: 'Select all' }).click();
    await expect.element(dialog()).not.toHaveTextContent('Pick at least one track');
  });

  test('Enter posts the album form and the ticked numbers', async () => {
    await inReview();

    track('2. مرحبا بالعالم').element().focus();
    await userEvent.keyboard(' ');
    await userEvent.keyboard('{Enter}');

    await expect.element(dialog()).toHaveTextContent('Downloading');
    expect(requestsTo(fetchMock, 'POST', '/api/add/album')).toEqual([{ id: 'al1', form: album.form, tracks: [1, 3] }]);
  });

  test('the progress screen shows the title and the bar from job events', async () => {
    await inReview();
    await userEvent.keyboard('{Enter}');
    await expect.element(dialog()).toHaveTextContent('Downloading');

    FakeEventSource.instances.at(-1)?.emit('job', JSON.stringify({ id: 'b1', kind: 'addalbum', state: 'running', data: { done: 1, total: 3, title: 'Blue in Green' } }));

    await expect.element(dialog()).toHaveTextContent('Downloading Blue in Green…');
    await expect.element(dialog()).toHaveTextContent('1/3');
    await expect.element(dialog()).toHaveTextContent('esc cancels.');
    await expect.element(dialog().getByRole('progressbar')).toHaveAttribute('aria-valuenow', '1');
  });

  test('Esc while adding asks the server to stop the job', async () => {
    await inReview();
    await userEvent.keyboard('{Enter}');
    await vi.waitFor(() => {
      expect(useAdd.getState().job?.id).toBe('b1');
    });

    await userEvent.keyboard('{Escape}');

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'DELETE', '/api/jobs/b1')).toHaveLength(1);
    });
  });

  test('Esc in the review discards the album', async () => {
    await inReview();

    await userEvent.keyboard('{Escape}');

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'DELETE', '/api/add/al1')).toHaveLength(1);
    });
  });

  test('the first field is focused when the review opens', async () => {
    await inReview();

    await expect.element(dialog().getByRole('textbox', { name: 'Album', exact: true })).toHaveFocus();
  });
});
