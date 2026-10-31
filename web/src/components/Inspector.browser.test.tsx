import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { commands, page, userEvent } from 'vitest/browser';
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

declare module 'vitest/browser' {
  interface BrowserCommands {
    serveCovers: () => Promise<void>;
  }
}

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

function valueOf(label: string) {
  return pane().getByTestId(`value-${label}`);
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
  test('enter edits a field and escape cancels', async () => {
    await showApp();
    await intoFields(keys.soWhat);

    await userEvent.keyboard('{Enter}');
    const input = pane().getByRole('textbox', { name: 'Title' });
    await expect.element(input).toHaveValue('So What');
    await expect.element(input).toHaveFocus();

    await userEvent.keyboard(' (take 2)');
    await expect.element(input).toHaveValue('So What (take 2)');
    await userEvent.keyboard('{Escape}');

    await expect.element(input).not.toBeInTheDocument();
    await expect.element(valueOf('title')).toHaveTextContent('So What');
    expect(useApp.getState()).toMatchObject({ edit: null, focus: 'inspector' });
    expect(requestsTo(fetchMock, 'PATCH', '/api/tracks/sowhat')).toEqual([]);
  });

  test('a mixed field opens empty and blank keeps every value', async () => {
    await showApp();
    await intoFields(keys.kindOfBlue);
    await userEvent.keyboard('{ArrowDown}');
    await expect.element(valueOf('genre')).toHaveTextContent('(mixed)');

    await userEvent.keyboard('{Enter}');
    const input = pane().getByRole('textbox', { name: 'Genre' });
    await expect.element(input).toHaveValue('');
    await expect.element(input).toHaveAttribute('placeholder', '(mixed)');
    await userEvent.keyboard('{Enter}');

    await expect.element(input).not.toBeInTheDocument();
    await expect.element(valueOf('genre')).toHaveTextContent('(mixed)');
    expect(requestsTo(fetchMock, 'PATCH', '/api/group')).toEqual([]);
  });

  test('enter saves what was typed and the field closes', async () => {
    serve(({ method }) => (method === 'PATCH' ? jsonResponse(changeResponse({ message: 'Saved', reveal: 'sowhat' })) : undefined));
    await showApp();
    await intoFields(keys.soWhat);

    await userEvent.keyboard('{Enter}');
    await userEvent.fill(pane().getByRole('textbox', { name: 'Title' }), 'So When');
    await userEvent.keyboard('{Enter}');

    await expect.element(pane().getByRole('textbox')).not.toBeInTheDocument();
    expect(requestsTo(fetchMock, 'PATCH', '/api/tracks/sowhat')).toEqual([{ field: 'title', text: 'So When', second: '' }]);
  });

  test("the server's refusal is shown under the field, which stays open", async () => {
    serve(({ method }) => (method === 'PATCH' ? jsonResponse({ error: 'The title cannot be empty' }, 422) : undefined));
    await showApp();
    await intoFields(keys.soWhat);

    await userEvent.keyboard('{Enter}');
    await userEvent.fill(pane().getByRole('textbox', { name: 'Title' }), '');
    await userEvent.keyboard('{Enter}');

    await expect.element(pane().getByRole('alert')).toHaveTextContent('The title cannot be empty');
    await expect.element(pane().getByRole('textbox', { name: 'Title' })).toBeVisible();

    await userEvent.keyboard('x');
    await expect.element(pane().getByRole('alert')).not.toBeInTheDocument();
  });

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

  test('- typed in a field is text, not the key that closes the tree', async () => {
    await showApp();
    await intoFields(keys.soWhat);
    const open = [...useApp.getState().expanded];

    await userEvent.keyboard('{Enter}');
    await userEvent.keyboard('-');

    await expect.element(pane().getByRole('textbox', { name: 'Title' })).toHaveValue('So What-');
    expect([...useApp.getState().expanded]).toEqual(open);
  });

  test('the footer shows the keys of the field being typed in', async () => {
    await showApp();
    await intoFields(keys.soWhat);

    await userEvent.keyboard('{Enter}');
    await expect.poll(hints).toEqual(['enter save', 'esc cancel']);

    await userEvent.keyboard('{Escape}{ArrowDown}{ArrowDown}{Enter}');
    await expect.poll(hints).toEqual(['enter save', 'tab next', 'esc cancel']);
  });

  test('clicking a field chooses it, and clicking it again edits it', async () => {
    await showApp();
    await select(keys.soWhat);

    await valueOf('artist').click();
    expect(useApp.getState()).toMatchObject({ focus: 'inspector', field: 1, edit: null });

    await valueOf('artist').click();
    await expect.element(pane().getByRole('textbox', { name: 'Artist' })).toHaveValue('Miles Davis');
  });

  test('the hint of the field under the cursor is shown under it', async () => {
    await showApp();
    await intoFields(keys.soWhat);

    await userEvent.keyboard('{ArrowDown}{ArrowDown}{ArrowDown}');

    await expect.element(pane().getByText('e.g. 1999, 1999-05 or 1999-05-17 · empty if unknown')).toBeVisible();
  });
});

describe('the tabs of a track', () => {
  test('the number keys and the arrows turn the tabs, and ← on the first goes back to the tree', async () => {
    await showApp();
    await intoFields(keys.soWhat);
    const tab = (name: string) => pane().getByRole('tab', { name });

    await expect.element(tab('1 Details')).toHaveAttribute('aria-selected', 'true');
    await userEvent.keyboard('3');
    await expect.element(tab('3 Latin')).toHaveAttribute('aria-selected', 'true');
    await userEvent.keyboard('{ArrowRight}');
    await expect.element(tab('4 Device')).toHaveAttribute('aria-selected', 'true');
    await userEvent.keyboard('{ArrowLeft}{ArrowLeft}{ArrowLeft}');
    await expect.element(tab('1 Details')).toHaveAttribute('aria-selected', 'true');
    expect(useApp.getState().focus).toBe('inspector');

    await userEvent.keyboard('{ArrowLeft}');
    expect(useApp.getState().focus).toBe('tree');
  });

  test('clicking a tab turns to it', async () => {
    await showApp();
    await select(keys.soWhat);

    await pane().getByRole('tab', { name: '2 Audio' }).click();

    await expect.element(valueOf('cutStart')).toHaveTextContent('0:00');
  });

  test('the device tab previews with the chosen profile', async () => {
    await showApp();
    await intoFields(keys.soWhat);
    const device = pane().getByRole('tab', { name: /Device$/ });
    await device.click();
    await expect.element(device).toHaveAttribute('aria-selected', 'true');
    const profile = pane().getByRole('combobox', { name: 'Profile' });

    await expect.element(profile).toHaveValue('wolfgang-wmp01');
    await expect.element(pane().getByText('wolfgang-wmp01/Folder')).toBeVisible();
    await expect.element(pane().getByText('a warning for wolfgang-wmp01')).toBeVisible();
    expect(
      page
        .getByRole('option')
        .elements()
        .map((option) => option.textContent),
    ).toEqual(['Generic', 'Wolfgang WMP01', 'My Player']);

    await profile.selectOptions('my-player');

    await expect.element(pane().getByText('my-player/Folder')).toBeVisible();
    await expect.element(pane().getByText('a warning for my-player')).toBeVisible();
    expect(requestsTo(fetchMock, 'GET', '/api/tracks/sowhat?profile=my-player')).toHaveLength(1);

    await select(keys.blue);
    await expect.poll(() => requestsTo(fetchMock, 'GET', '/api/tracks/blue?profile=my-player').length > 0).toBe(true);
  });

  test('the Audio tab draws what the cuts keep', async () => {
    await showApp();
    await select(keys.soWhat);
    await userEvent.keyboard('{Tab}2');

    await expect.element(pane().getByRole('img', { name: '0:01 of 0:02' })).toBeVisible();
  });
});

describe('hearing a cut', () => {
  test('p on a cut starts the preview audio', async () => {
    const play = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined);
    serve(({ url }) =>
      url.pathname === '/api/tracks/sowhat/preview'
        ? new Response(new Blob([new Uint8Array([0xff, 0xfb, 0x90, 0x00])], { type: 'audio/mpeg' }), { headers: { 'Content-Type': 'audio/mpeg' } })
        : undefined,
    );
    await showApp();
    await intoFields(keys.soWhat);
    await userEvent.keyboard('2');

    await userEvent.keyboard('p');

    await expect.poll(() => document.querySelector('audio')?.getAttribute('src') ?? '').toMatch(/^blob:/);
    await expect.poll(() => play.mock.calls.length).toBe(1);
    expect(requestsTo(fetchMock, 'GET', '/api/tracks/sowhat/preview?field=cutStart')).toHaveLength(1);
    await expect.poll(hints).toContain('p stop');

    await userEvent.keyboard('p');

    await expect.poll(() => document.querySelector('audio')).toBeNull();
    await expect.poll(hints).toContain('p hear');
  });

  test('the preview stops when it has played to its end', async () => {
    vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined);
    await showApp();
    useApp.setState({ playing: { id: 'sowhat', field: 'cutStart', src: 'blob:nothing' } });

    await expect.poll(() => document.querySelector('audio')).not.toBeNull();
    document.querySelector('audio')?.dispatchEvent(new Event('ended'));

    await expect.poll(() => useApp.getState().playing).toBeNull();
  });
});

describe('what the pane shows', () => {
  test('a track: its title, its fields, where its file goes, and its names in their own direction', async () => {
    await showApp();
    await select(keys.watani);

    await expect.element(pane().getByRole('heading', { level: 3 })).toHaveTextContent('وطني');
    const names = [...pane().element().querySelectorAll('bdi')];
    expect(names.filter((name) => name.textContent === 'وطني').map((name) => name.closest('[data-testid], h3')?.tagName)).toEqual(['H3', 'DIV']);
    expect(names.every((name) => name.getAttribute('dir') === 'auto')).toBe(true);
    await expect.element(pane().getByText('فيروز/Songs/01 - وطني.mp3')).toBeVisible();
    await expect.element(valueOf('cover')).toHaveTextContent('none');
  });

  test('a worked-out Latin form is dim and a hand-set one is not', async () => {
    await showApp();
    await intoFields(keys.soWhat);
    await userEvent.keyboard('3');

    await expect.element(valueOf('latinArtist')).toHaveClass(/dim/);
    await expect.element(valueOf('latinTitle')).not.toHaveClass(/dim/);
    await expect.element(valueOf('latinGenre')).toHaveTextContent('—');
  });

  test('the cover is shown for a track that has one, and nothing for one that has none', async () => {
    serve(({ method, url }) =>
      method === 'GET' && url.pathname === '/api/tracks/sowhat' ? jsonResponse({ ...soWhat(), coverSha: 'cc1a610a49c5c3b87b956373c45d3339' }) : undefined,
    );
    await commands.serveCovers();
    await showApp();
    await select(keys.soWhat);

    const cover = pane().getByRole('img', { name: 'Cover' });
    await expect.element(cover).toHaveAttribute('src', '/api/tracks/sowhat/cover?v=cc1a610a49c5c3b8');
    await expect.poll(() => (cover.element() as HTMLImageElement).naturalWidth).toBe(1);

    await select(keys.watani);
    await expect.element(cover).not.toBeInTheDocument();
  });

  test('an album: its fields, what is inside it, and the title of the pane', async () => {
    await showApp();
    await select(keys.kindOfBlue);

    await expect.element(page.getByRole('heading', { level: 2, name: 'Album' })).toBeVisible();
    await expect.element(pane().getByText('Miles Davis · 2 tracks · 0:05')).toBeVisible();
    await expect.element(pane().getByText('all 2 tracks · one undo step')).toBeVisible();
    await expect.element(pane().getByRole('button', { name: /So What/ })).toBeVisible();

    await pane().getByRole('button', { name: /Blue in Green/ }).click();

    await expect.poll(() => useApp.getState().cursor).toBe(keys.blue);
  });

  test('the Needs work group lists what each track lacks', async () => {
    await showApp();
    await select(keys.needs);

    await expect.element(pane().getByRole('button', { name: /Rough Mix/ })).toHaveTextContent('no title, no artist');
  });

  test('nothing selected is said when nothing in the tree is shown', async () => {
    useApp.setState({ filter: 'nothing matches this' });
    await render(<App />);

    await expect.element(pane().getByText('Nothing selected')).toBeVisible();
  });

  test('details that cannot be had are said in the pane', async () => {
    serve(({ url }) => (url.pathname === '/api/tracks/sowhat' ? jsonResponse({ error: 'the track is gone' }, 404) : undefined));
    await showApp();
    const { lib, expanded } = useApp.getState();
    const row = rowByKey(lib, keys.soWhat);
    if (lib === null || row === undefined) {
      throw new Error('no So What');
    }
    useApp.setState({ expanded: new Set([...expanded, ...ancestorKeys(lib.view.rows, row)]) });
    pointRow(keys.soWhat);

    await expect.element(pane().getByText('the track is gone')).toBeVisible();
  });
});
