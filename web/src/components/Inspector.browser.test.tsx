import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { page, userEvent } from 'vitest/browser';
import { render } from 'vitest-browser-react';
import type { PeekView, RowView, TrackView } from '../api/types.gen';
import { App } from '../App';
import { pointRow } from '../state/actions';
import { useApp } from '../state/store';
import { ancestorKeys, rowByKey } from '../state/tree';
import {
  changeResponse,
  fieldView,
  FakeEventSource,
  infoView,
  jsonResponse,
  requestsTo,
  sampleLibrary,
  stubServer,
  withDevice,
  type StubRoute,
} from '../testing/fakes';

const sep = '\u001f';
const keys = {
  needs: 'needs',
  miles: `artist${sep}Miles Davis`,
  kindOfBlue: `album${sep}Miles Davis${sep}Kind of Blue`,
  soWhat: `track${sep}sowhat`,
  blue: `track${sep}blue`,
  watani: `track${sep}watani`,
};

function soWhat(): TrackView {
  return {
    id: 'sowhat',
    title: 'So What',
    identified: true,
    tabs: [
      {
        id: 'details',
        name: 'Details',
        fields: [
          fieldView('title', { label: 'Title', text: 'So What', edit: 'So What' }),
          fieldView('artist', { label: 'Artist', text: 'Miles Davis', edit: 'Miles Davis' }),
          fieldView('track', { label: 'Track', text: '1 of 5', edit: '1', second: '5' }),
          fieldView('released', { label: 'Released', hint: 'e.g. 1999, 1999-05 or 1999-05-17 · empty if unknown' }),
          fieldView('cover', { label: 'Cover', text: '500×500 JPEG', empty: 'none' }),
        ],
      },
      {
        id: 'audio',
        name: 'Audio',
        fields: [fieldView('cutStart', { label: 'Cut start', text: '0:00', empty: '' }), fieldView('cutEnd', { label: 'Cut end', text: '0:00', empty: '' })],
        bar: { total: 2_000_000_000, start: 0, end: 500_000_000, kept: '0:01 of 0:02' },
      },
      {
        id: 'latin',
        name: 'Latin',
        notes: ['Dim forms are worked out from the names.'],
        fields: [
          fieldView('latinTitle', { label: 'Title', text: 'So What', edit: 'So What' }),
          fieldView('latinArtist', { label: 'Artist', text: 'Miles Davis', edit: 'Miles Davis', dim: true }),
          fieldView('latinGenre', { label: 'Genre', editable: false, dim: true }),
        ],
      },
    ],
    path: 'Miles Davis/Kind of Blue/01 - So What.mp3',
    dates: 'added 2026-10-31',
    about: '0:02 plays · identified',
    reveal: [],
  };
}

function watani(): TrackView {
  return {
    id: 'watani',
    title: 'وطني',
    identified: true,
    tabs: [{ id: 'details', name: 'Details', fields: [fieldView('title', { label: 'Title', text: 'وطني', edit: 'وطني' }), fieldView('cover', { label: 'Cover', empty: 'none' })] }],
    path: 'فيروز/Songs/01 - وطني.mp3',
    dates: '',
    about: '',
    reveal: [],
  };
}

function kindOfBlue(): PeekView {
  return {
    kind: 'album',
    title: 'Album',
    label: 'Kind of Blue',
    editable: true,
    info: ['Miles Davis · 2 tracks · 0:05'],
    fields: [
      fieldView('album', { label: 'Name', text: 'Kind of Blue', edit: 'Kind of Blue' }),
      fieldView('genre', { label: 'Genre', mixed: true, empty: '(mixed)', hint: 'the tracks differ · empty keeps them · - clears' }),
    ],
    note: 'all 2 tracks · one undo step',
    list: [
      { key: keys.soWhat, left: '1  So What', cols: ['0:02'] },
      { key: keys.blue, left: '3  Blue in Green', cols: ['0:03'] },
    ],
  };
}

function needsWork(): PeekView {
  return {
    kind: 'group',
    title: 'Needs work',
    label: 'Needs work',
    editable: false,
    info: ['These tracks have not been identified yet.'],
    list: [{ key: `track${sep}rough`, left: 'Rough Mix', cols: ['no title, no artist', '0:05'] }],
  };
}

const views: StubRoute = ({ method, url }) => {
  const profile = url.searchParams.get('profile');
  const withProfile = (view: TrackView) => jsonResponse(profile === null ? view : withDevice(view, profile));
  if (method === 'GET' && url.pathname === '/api/tracks/sowhat') {
    return withProfile(soWhat());
  }
  if (method === 'GET' && url.pathname === '/api/tracks/watani') {
    return withProfile(watani());
  }
  if (method === 'GET' && url.pathname === '/api/peek' && url.searchParams.get('key') === keys.kindOfBlue) {
    return jsonResponse(kindOfBlue());
  }
  if (method === 'GET' && url.pathname === '/api/peek' && url.searchParams.get('key') === keys.needs) {
    return jsonResponse(needsWork());
  }
  return undefined;
};

let fetchMock: ReturnType<typeof stubServer>;

function serve(route: StubRoute = () => undefined): void {
  fetchMock = stubServer(infoView(), sampleLibrary(), { route: (request) => route(request) ?? views(request) });
}

async function showApp(): Promise<void> {
  await render(<App />);
  await expect.element(page.getByRole('treeitem').first()).toBeVisible();
}

async function select(key: string): Promise<void> {
  const { lib, expanded } = useApp.getState();
  const row = rowByKey(lib, key);
  if (lib === null || row === undefined) {
    throw new Error(`no row ${key}`);
  }
  useApp.setState({ expanded: new Set([...expanded, ...ancestorKeys(lib.view.rows, row)]) });
  pointRow(key);
  await expect.poll(() => shows(row)).toBe(true);
}

function shows(row: RowView): boolean {
  const d = useApp.getState().detail;
  if (d === null) {
    return false;
  }
  return 'tabs' in d ? d.id === row.trackId : d.label === row.label;
}

async function intoFields(key: string): Promise<void> {
  await select(key);
  await userEvent.keyboard('{Tab}');
  await expect.poll(() => useApp.getState().focus).toBe('inspector');
}

function pane() {
  return page.getByRole('region', { name: 'Details' });
}

function hints(): string[] {
  return page
    .getByRole('list', { name: 'Keys' })
    .getByRole('listitem')
    .elements()
    .map((item) => item.textContent.trim());
}

beforeEach(async () => {
  await page.viewport(1200, 800);
  localStorage.clear();
  useApp.setState(useApp.getInitialState(), true);
  FakeEventSource.instances = [];
  serve();
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('editing a field', () => {

  test('enter while a word is being composed does not save', async () => {
    await showApp();
    await intoFields(keys.soWhat);
    await userEvent.keyboard('{Enter}');
    const input = pane().getByRole('textbox', { name: 'Title' });
    await userEvent.fill(input, 'So When');

    input.element().dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', isComposing: true, bubbles: true, cancelable: true }));

    await expect.element(input).toBeVisible();
    expect(useApp.getState().edit).not.toBeNull();
    expect(requestsTo(fetchMock, 'PATCH', '/api/tracks/sowhat')).toEqual([]);
  });

  test('tab in the number of a track goes to the number of tracks, and back', async () => {
    serve(({ method }) => (method === 'PATCH' ? jsonResponse(changeResponse({ message: 'Saved' })) : undefined));
    await showApp();
    await intoFields(keys.soWhat);
    await userEvent.keyboard('{ArrowDown}{ArrowDown}{Enter}');
    const number = pane().getByRole('textbox', { name: 'Track', exact: true });
    const of = pane().getByRole('textbox', { name: 'Tracks on album' });
    await expect.element(number).toHaveValue('1');
    await expect.element(of).toHaveValue('5');

    await userEvent.keyboard('{Tab}');
    await expect.element(of).toHaveFocus();
    await userEvent.keyboard('{Backspace}6');
    await userEvent.keyboard('{Tab}');
    await expect.element(number).toHaveFocus();
    expect(useApp.getState().focus).toBe('inspector');

    await userEvent.keyboard('{Enter}');
    await expect.poll(() => requestsTo(fetchMock, 'PATCH', '/api/tracks/sowhat')).toEqual([{ field: 'track', text: '1', second: '6' }]);
  });

});

describe('setting a cover', () => {
  const setCover: StubRoute = ({ method, url }) => (method === 'PUT' && url.pathname === '/api/cover' ? jsonResponse(changeResponse({ message: 'Set the cover' })) : undefined);

  function picture(name: string, type: string): File {
    return new File([new Uint8Array([0x89, 0x50, 0x4e, 0x47])], name, { type });
  }

  function paste(file: File): boolean {
    const data = new DataTransfer();
    data.items.add(file);
    return document.dispatchEvent(new ClipboardEvent('paste', { clipboardData: data, bubbles: true, cancelable: true }));
  }

  test('enter on the Cover field opens a chooser of JPEG and PNG files, and the file chosen becomes the cover', async () => {
    serve(setCover);
    const click = vi.spyOn(HTMLInputElement.prototype, 'click').mockImplementation(() => undefined);
    await showApp();
    await intoFields(keys.soWhat);
    for (let i = 0; i < 4; i += 1) {
      await userEvent.keyboard('{ArrowDown}');
    }
    await expect.poll(hints).toContain('enter choose');

    await userEvent.keyboard('{Enter}');

    const chooser = click.mock.contexts[0];
    if (!(chooser instanceof HTMLInputElement)) {
      throw new Error('no file chooser was opened');
    }
    expect(chooser.type).toBe('file');
    expect(chooser.accept).toBe('image/jpeg,image/png');
    expect(useApp.getState().edit).toBeNull();
    const file = picture('front.png', 'image/png');
    const files = new DataTransfer();
    files.items.add(file);
    chooser.files = files.files;
    chooser.dispatchEvent(new Event('change', { bubbles: true }));

    await expect.poll(() => requestsTo(fetchMock, 'PUT', '/api/cover?ids=sowhat')).toEqual([file]);
    await expect.element(page.getByRole('status')).toHaveTextContent('Set the cover');
    await expect.poll(() => useApp.getState().busy).toBe(false);
    expect(chooser.value).toBe('');
  });

  test('a picture pasted with ctrl+v becomes the cover of the track under the cursor', async () => {
    serve(setCover);
    await showApp();
    await select(keys.soWhat);
    const file = picture('front.png', 'image/png');

    expect(paste(file)).toBe(false);

    await expect.poll(() => requestsTo(fetchMock, 'PUT', '/api/cover?paste=1&name=front.png&ids=sowhat')).toEqual([file]);
    await expect.poll(() => useApp.getState().busy).toBe(false);
  });

  test('a pasted file that is not a JPEG or a PNG is refused with a message', async () => {
    serve(setCover);
    await showApp();
    await select(keys.soWhat);

    paste(picture('front.webp', 'image/webp'));

    await expect.element(page.getByRole('status')).toHaveTextContent('A cover must be a JPEG or PNG image');
    expect(fetchMock.mock.calls.filter(([, init]) => init?.method === 'PUT')).toEqual([]);
  });

  test('a picture pasted while a dialog is open is left alone', async () => {
    serve(setCover);
    await showApp();
    await select(keys.soWhat);
    useApp.setState({ overlay: { kind: 'help' } });
    await expect.element(page.getByRole('dialog')).toBeVisible();

    expect(paste(picture('front.png', 'image/png'))).toBe(true);

    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(fetchMock.mock.calls.filter(([, init]) => init?.method === 'PUT')).toEqual([]);
  });

  test('a picture pasted while typing in a box is left to the box', async () => {
    serve(setCover);
    await showApp();
    await select(keys.soWhat);
    page.getByRole('textbox', { name: 'Filter' }).element().focus();

    expect(paste(picture('front.png', 'image/png'))).toBe(true);

    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(fetchMock.mock.calls.filter(([, init]) => init?.method === 'PUT')).toEqual([]);
  });
});
