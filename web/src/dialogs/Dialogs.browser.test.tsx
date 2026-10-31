import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { page, userEvent } from 'vitest/browser';
import { render } from 'vitest-browser-react';
import type { CopyChoice, LibraryResponse, SortChoice } from '../api/types.gen';
import { App } from '../App';
import { goToRow } from '../state/edit';
import { useApp } from '../state/store';
import { changeResponse, FakeEventSource, infoView, jsonResponse, requestsTo, sampleLibrary, stubServer, type StubRoute } from '../testing/fakes';
import { helpPages } from '../keys/keymap';

const sep = '\u001f';
const soWhat = `track${sep}sowhat`;
const blueInGreen = `track${sep}blue`;
const kindOfBlue = `album${sep}Miles Davis${sep}Kind of Blue`;
const longName = 'A rather long playlist name that goes on and on and on for a while';

const sortChoices: SortChoice[] = [
  { key: '', label: 'Tree', forward: '', backward: '' },
  { key: 'artist', label: 'Artist', forward: 'A → Z', backward: 'Z → A' },
  { key: 'album', label: 'Album', forward: 'A → Z', backward: 'Z → A' },
  { key: 'title', label: 'Title', forward: 'A → Z', backward: 'Z → A' },
];

const copyChoices: CopyChoice[] = [
  { key: 'cover', label: 'Cover', has: false, text: '' },
  { key: 'artist', label: 'Artist', has: true, text: 'Miles Davis' },
  { key: 'album', label: 'Album', has: true, text: 'Kind of Blue' },
];

function library(playlists = [
  { name: 'Late night', count: 1, length: '0:03' },
  { name: longName, count: 2, length: '0:07' },
]): LibraryResponse {
  const base = sampleLibrary();
  return { ...base, view: { ...base.view, playlists, sort: { ...base.view.sort, choices: sortChoices } } };
}

let fetchMock: ReturnType<typeof stubServer>;

const changes: StubRoute = ({ method, url }) => {
  if (method === 'GET' && url.pathname === '/api/tracks/sowhat/copyable') {
    return jsonResponse(copyChoices);
  }
  if (method === 'POST' && ['/api/playlists', '/api/playlists/add', '/api/copy'].includes(url.pathname)) {
    return jsonResponse(changeResponse({ message: 'Done' }));
  }
  return undefined;
};

async function openApp(lib: LibraryResponse = library(), route: StubRoute = changes): Promise<void> {
  fetchMock = stubServer(infoView(), lib, { route });
  await render(<App />);
  await expect.element(page.getByRole('treeitem').first()).toBeVisible();
}

async function onRow(key: string): Promise<void> {
  goToRow(key);
  page.getByRole('tree').element().focus();
  await expect.element(page.getByRole('tree').getByRole('treeitem', { selected: true })).toBeVisible();
}

function dialog() {
  return page.getByRole('dialog');
}

function options(): string[] {
  return dialog()
    .getByRole('option')
    .elements()
    .map((option) => [...option.children].map((cell) => cell.textContent.trim()).join('|'));
}

function hints(): string[] {
  return page
    .getByRole('list', { name: 'Keys' })
    .getByRole('listitem')
    .elements()
    .map((item) => item.textContent.trim());
}

function libraryRequests(): string[] {
  return fetchMock.mock.calls.flatMap(([input]) => (typeof input === 'string' && input.startsWith('/api/library') ? [input] : []));
}

function askFor(onYes: () => void): void {
  useApp.setState({ overlay: { kind: 'ask', head: 'Remove So What?', body: ['It goes from the library.', '• So What'], yes: 'Remove', onYes } });
}

beforeEach(() => {
  localStorage.clear();
  useApp.setState(useApp.getInitialState(), true);
  FakeEventSource.instances = [];
});

afterEach(async () => {
  await new Promise((done) => setTimeout(done, 80));
  vi.unstubAllGlobals();
});

describe('the dialog shell', () => {
  test('escape closes and focus returns', async () => {
    await openApp();
    await onRow(soWhat);
    const tree = page.getByRole('tree').element();

    await userEvent.keyboard('?');

    await expect.element(dialog()).toBeVisible();
    await expect.element(dialog()).toHaveAttribute('aria-modal', 'true');
    await expect.element(dialog()).toHaveAccessibleName('Keys');
    expect(dialog().element().contains(document.activeElement)).toBe(true);

    await userEvent.keyboard('{Escape}');

    await expect.element(dialog()).not.toBeInTheDocument();
    expect(document.activeElement).toBe(tree);
  });

  test('the title is drawn in the border', async () => {
    await openApp();
    await onRow(soWhat);
    await userEvent.keyboard('?');

    const box = dialog().element().getBoundingClientRect();
    const title = dialog().getByRole('heading', { name: 'Keys' }).element().getBoundingClientRect();

    expect(title.top).toBeLessThan(box.top);
    expect(title.bottom).toBeGreaterThan(box.top);
  });

  test('a click on the backdrop closes the dialog and a click inside does not', async () => {
    await openApp();
    await onRow(soWhat);
    await userEvent.keyboard('?');

    await dialog().getByRole('heading').click();
    await expect.element(dialog()).toBeVisible();

    const box = dialog().element().getBoundingClientRect();
    await userEvent.click(page.getByTestId('backdrop'), { position: { x: 2, y: Math.max(2, box.top - 20) } });

    await expect.element(dialog()).not.toBeInTheDocument();
  });

  test('tab keeps the focus inside the dialog', async () => {
    await openApp();
    await onRow(soWhat);
    askFor(vi.fn());
    await expect.element(dialog()).toBeVisible();

    for (let i = 0; i < 5; i += 1) {
      await userEvent.keyboard('{Tab}');
      expect(dialog().element().contains(document.activeElement)).toBe(true);
    }
    await userEvent.keyboard('{Shift>}{Tab}{/Shift}');
    expect(dialog().element().contains(document.activeElement)).toBe(true);
  });

  test('a click outside the dialog cannot take the focus out of it', async () => {
    await openApp();
    await onRow(soWhat);
    askFor(vi.fn());
    await expect.element(dialog()).toBeVisible();

    page.getByRole('tree').element().focus();

    expect(dialog().element().contains(document.activeElement)).toBe(true);
  });

  test('the footer shows the keys of the open dialog and the tree keys again when it closes', async () => {
    await openApp();
    await onRow(soWhat);
    const treeHints = hints();
    expect(treeHints).toContain('? help');

    askFor(vi.fn());

    await expect.poll(hints).toEqual(['y/enter Remove', 'esc close']);

    await userEvent.keyboard('{Escape}');

    await expect.poll(hints).toEqual(treeHints);
  });

  test('escape in a dialog closes it and does not also clear the marks or the filter', async () => {
    await openApp();
    await onRow(soWhat);
    await userEvent.keyboard(' ');
    await expect.poll(() => useApp.getState().marks.size).toBe(1);

    await userEvent.keyboard('?');
    await expect.element(dialog()).toBeVisible();
    await userEvent.keyboard('{Escape}');

    await expect.element(dialog()).not.toBeInTheDocument();
    expect(useApp.getState().marks.size).toBe(1);

    useApp.setState({ filter: 'blue' });
    await userEvent.keyboard('?');
    await userEvent.keyboard('{Escape}');

    await expect.element(dialog()).not.toBeInTheDocument();
    expect(useApp.getState().filter).toBe('blue');
  });

  test('after a dialog closes the keys move the tree again', async () => {
    await openApp();
    await onRow(soWhat);
    await userEvent.keyboard('?');
    await userEvent.keyboard('j');
    expect(useApp.getState().cursor).toBe(soWhat);

    await userEvent.keyboard('{Escape}');
    await userEvent.keyboard('j');

    expect(useApp.getState().cursor).toBe(blueInGreen);
  });
});

describe('ask', () => {
  test('ask goes ahead only on y or enter', async () => {
    await openApp();
    await onRow(soWhat);
    const onYes = vi.fn();
    askFor(onYes);

    await expect.element(dialog()).toHaveAccessibleName('Are you sure?');
    await expect.element(dialog().getByText('Remove So What?')).toBeVisible();
    for (const line of ['Remove So What?', 'It goes from the library.', '• So What']) {
      const bdi = dialog().getByText(line).element();
      expect(bdi.tagName).toBe('BDI');
      expect(bdi.getAttribute('dir')).toBe('auto');
    }

    await userEvent.keyboard('a');
    await userEvent.keyboard('x');
    await userEvent.keyboard('j');
    await expect.element(dialog()).toBeVisible();
    expect(onYes).not.toHaveBeenCalled();

    await userEvent.keyboard('y');
    await expect.element(dialog()).not.toBeInTheDocument();
    expect(onYes).toHaveBeenCalledTimes(1);

    askFor(onYes);
    await expect.element(dialog()).toBeVisible();
    await userEvent.keyboard('{Enter}');
    await expect.element(dialog()).not.toBeInTheDocument();
    expect(onYes).toHaveBeenCalledTimes(2);

    askFor(onYes);
    await expect.element(dialog()).toBeVisible();
    await userEvent.keyboard('{Escape}');
    await expect.element(dialog()).not.toBeInTheDocument();
    expect(onYes).toHaveBeenCalledTimes(2);
  });
});

describe('help', () => {
  test('? in the help closes it and does not open it again', async () => {
    await openApp();
    await onRow(soWhat);
    await userEvent.keyboard('?');
    await expect.element(dialog()).toBeVisible();

    await userEvent.keyboard('?');

    await expect.element(dialog()).not.toBeInTheDocument();
  });

  test('the numbers and tab turn the pages of the help, and the note on stopping mm is under them', async () => {
    await openApp();
    await onRow(soWhat);
    await userEvent.keyboard('?');
    const selected = () => dialog().getByRole('tab', { selected: true }).element().textContent;
    const pages = helpPages();

    await expect.poll(selected).toBe(`1 ${pages[0]?.page ?? ''}`);
    await userEvent.keyboard('3');
    await expect.poll(selected).toBe(`3 ${pages[2]?.page ?? ''}`);
    await userEvent.keyboard('{Tab}');
    await expect.poll(selected).toBe(`4 ${pages[3]?.page ?? ''}`);
    await userEvent.keyboard('{Tab}');
    await expect.poll(selected).toBe(`1 ${pages[0]?.page ?? ''}`);
    await expect.element(dialog().getByText('Ctrl+C in the terminal where mm runs stops the program.')).toBeVisible();
  });
});

describe('the sort menu', () => {
  test('the sort menu picks by number', async () => {
    await openApp();
    await onRow(soWhat);

    await userEvent.keyboard('S');

    await expect.element(dialog()).toHaveAccessibleName('Sort');
    expect(options()).toEqual(['1|Tree|', '2|Artist|A → Z', '3|Album|A → Z', '4|Title|A → Z']);

    await userEvent.keyboard('4');

    await expect.element(dialog()).not.toBeInTheDocument();
    await expect.poll(libraryRequests).toContain('/api/library?sort=title');
  });

  test('r reverses the order the menu picks', async () => {
    await openApp();
    await onRow(soWhat);

    await userEvent.keyboard('S');
    await userEvent.keyboard('r');

    await expect.poll(options).toEqual(['1|Tree|', '2|Artist|Z → A', '3|Album|Z → A', '4|Title|Z → A']);
    await userEvent.keyboard('{ArrowDown}');
    await userEvent.keyboard('{ArrowDown}');
    await userEvent.keyboard('{Enter}');

    await expect.poll(libraryRequests).toContain('/api/library?sort=album&reverse=1');
  });
});

describe('the copy list', () => {
  test('the copy list cannot tick an attribute the track lacks', async () => {
    await openApp();
    await onRow(soWhat);

    await userEvent.keyboard('c');

    await expect.element(dialog()).toHaveAccessibleName('Copy from So What');
    expect(options()).toEqual(['|Cover|—', '[x]|Artist|Miles Davis', '[x]|Album|Kind of Blue']);
    await expect.element(dialog().getByRole('option', { selected: true })).toHaveTextContent('Artist');

    await userEvent.keyboard('{ArrowUp}');
    await userEvent.keyboard(' ');
    expect(options()).toEqual(['|Cover|—', '[x]|Artist|Miles Davis', '[x]|Album|Kind of Blue']);

    await userEvent.keyboard('{ArrowDown}');
    await userEvent.keyboard(' ');
    expect(options()).toEqual(['|Cover|—', '[ ]|Artist|Miles Davis', '[x]|Album|Kind of Blue']);

    await userEvent.keyboard('{Enter}');

    await expect.element(dialog()).not.toBeInTheDocument();
    expect(requestsTo(fetchMock, 'POST', '/api/copy')).toEqual([{ id: 'sowhat', attrs: ['album'] }]);
  });

  test('the copy list asks for a tick before it copies nothing', async () => {
    await openApp();
    await onRow(soWhat);
    await userEvent.keyboard('c');
    await expect.element(dialog()).toBeVisible();

    await userEvent.keyboard(' ');
    await userEvent.keyboard('j');
    await userEvent.keyboard(' ');
    await dialog().getByRole('option').first().click({ force: true });
    expect(options()).toEqual(['|Cover|—', '[ ]|Artist|Miles Davis', '[ ]|Album|Kind of Blue']);
    await userEvent.keyboard('{Enter}');

    await expect.element(dialog().getByRole('alert')).toHaveTextContent('Tick what to copy');
    expect(requestsTo(fetchMock, 'POST', '/api/copy')).toEqual([]);
  });

  test('c on what is not a track says to choose a track', async () => {
    await openApp();
    await onRow(kindOfBlue);

    await userEvent.keyboard('c');

    await expect.element(page.getByRole('status')).toHaveTextContent('Select a track to copy from');
    expect(dialog().query()).toBeNull();
  });
});

describe('the playlist picker', () => {
  test('n turns the picker into a name input', async () => {
    await openApp();
    await onRow(soWhat);

    await userEvent.keyboard('p');

    await expect.element(dialog()).toHaveAccessibleName('Add to a playlist');
    expect(options()).toEqual(['|Late night|1', `|${longName}|2`, '|New playlist…|']);

    await userEvent.keyboard('n');

    const name = dialog().getByRole('textbox', { name: 'Name of the playlist' });
    await expect.element(name).toHaveFocus();
    expect(dialog().getByRole('listbox').query()).toBeNull();
    await expect.poll(hints).toEqual(['enter make it', 'esc back']);

    await userEvent.keyboard('nqjk');
    await expect.element(name).toHaveValue('nqjk');
    await expect.element(dialog()).toBeVisible();

    await userEvent.keyboard('{Escape}');
    await expect.element(dialog().getByRole('listbox')).toBeVisible();

    await userEvent.keyboard('n');
    await userEvent.fill(name, 'Road');
    await userEvent.keyboard('{Enter}');

    await expect.element(dialog()).not.toBeInTheDocument();
    expect(requestsTo(fetchMock, 'POST', '/api/playlists')).toEqual([{ name: 'Road', ids: ['sowhat'] }]);
  });

  test('enter on a playlist adds the album under the cursor to it', async () => {
    await openApp();
    await onRow(kindOfBlue);

    await userEvent.keyboard('p');
    await expect.element(dialog().getByText('Adding 2 tracks')).toBeVisible();
    await userEvent.keyboard('{Enter}');

    await expect.element(dialog()).not.toBeInTheDocument();
    expect(requestsTo(fetchMock, 'POST', '/api/playlists/add')).toEqual([{ name: 'Late night', ids: ['sowhat', 'blue'] }]);
    await expect.poll(libraryRequests).toHaveLength(2);
  });

  test('the marked tracks go to the playlist in the order of the tree', async () => {
    await openApp();
    await onRow(soWhat);
    await userEvent.keyboard(' ');
    await onRow(blueInGreen);
    await userEvent.keyboard(' ');

    await userEvent.keyboard('p');
    await userEvent.keyboard('j');
    await userEvent.keyboard('{Enter}');

    await expect.element(dialog()).not.toBeInTheDocument();
    expect(requestsTo(fetchMock, 'POST', '/api/playlists/add')).toEqual([{ name: longName, ids: ['blue', 'sowhat'] }]);
  });

  test('with no playlists the picker opens on the name input and esc closes it', async () => {
    await openApp(library([]));
    await onRow(soWhat);

    await userEvent.keyboard('p');

    await expect.element(dialog().getByRole('textbox', { name: 'Name of the playlist' })).toHaveFocus();
    await expect.poll(hints).toEqual(['enter make it', 'esc close']);
    await userEvent.keyboard('{Escape}');

    await expect.element(dialog()).not.toBeInTheDocument();
  });

  test('an enter that ends a composed word does not make the playlist', async () => {
    await openApp(library([]));
    await onRow(soWhat);
    await userEvent.keyboard('p');
    const name = dialog().getByRole('textbox', { name: 'Name of the playlist' });
    await userEvent.fill(name, 'ليل');

    name.element().dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', isComposing: true, bubbles: true, cancelable: true }));

    await expect.element(dialog()).toBeVisible();
    expect(requestsTo(fetchMock, 'POST', '/api/playlists')).toEqual([]);
  });

  test('a playlist with no name is refused in the dialog', async () => {
    await openApp(library([]));
    await onRow(soWhat);
    await userEvent.keyboard('p');

    await userEvent.keyboard('{Enter}');

    await expect.element(dialog().getByRole('alert')).toHaveTextContent('Type a name');
    expect(requestsTo(fetchMock, 'POST', '/api/playlists')).toEqual([]);
  });

  test("the server's refusal is shown in the picker, which stays open", async () => {
    const refuse: StubRoute = ({ method, url }) =>
      method === 'POST' && url.pathname === '/api/playlists/add' ? jsonResponse({ error: 'the playlist is gone' }, 400) : undefined;
    await openApp(library(), refuse);
    await onRow(soWhat);
    await userEvent.keyboard('p');

    await userEvent.keyboard('{Enter}');

    await expect.element(dialog().getByRole('alert')).toHaveTextContent('the playlist is gone');
    await expect.element(dialog().getByRole('listbox')).toBeVisible();
  });

  test('p on the Playlists group says what to choose', async () => {
    await openApp();
    await onRow('playlists');

    await userEvent.keyboard('p');

    await expect.element(page.getByRole('status')).toHaveTextContent('Select tracks to add to a playlist');
    expect(dialog().query()).toBeNull();
  });
});
