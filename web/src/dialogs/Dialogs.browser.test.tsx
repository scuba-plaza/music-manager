import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { page, userEvent } from 'vitest/browser';
import { render } from 'vitest-browser-react';
import type { CopyChoice, LibraryResponse, SortChoice } from '../api/types.gen';
import { App } from '../App';
import { goToRow } from '../state/edit';
import { useApp } from '../state/store';
import { changeResponse, FakeEventSource, infoView, jsonResponse, requestsTo, sampleLibrary, stubServer, type StubRoute } from '../testing/fakes';

const sep = '\u001f';
const soWhat = `track${sep}sowhat`;
const blueInGreen = `track${sep}blue`;
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

  test('enter on the focused Cancel button cancels and does not go ahead', async () => {
    await openApp();
    await onRow(soWhat);
    const onYes = vi.fn();
    askFor(onYes);
    await expect.element(dialog()).toBeVisible();

    await userEvent.keyboard('{Tab}{Tab}');
    await expect.element(dialog().getByRole('button', { name: 'Cancel' })).toHaveFocus();
    await userEvent.keyboard('{Enter}');

    await expect.element(dialog()).not.toBeInTheDocument();
    expect(onYes).not.toHaveBeenCalled();
  });
});

describe('the playlist picker', () => {

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

});

describe('the bulk form', () => {

  const bulk: StubRoute = ({ method, url, body }) => {
    if (method === 'POST' && url.pathname === '/api/bulk/preview') {
      const values = (body as { values: Record<string, string> }).values;
      return jsonResponse({ summary: values.album === '' ? '' : `Album → ${values.album ?? ''}`, changed: 2 });
    }
    if (method === 'POST' && url.pathname === '/api/bulk') {
      return jsonResponse(changeResponse({ message: 'Changed 2 tracks' }));
    }
    return undefined;
  };

  async function openForm(route: StubRoute = bulk): Promise<void> {
    await openApp(library(), route);
    await onRow(soWhat);
    await userEvent.keyboard(' ');
    await userEvent.keyboard(' ');
    await expect.poll(() => useApp.getState().marks.size).toBe(2);
    await userEvent.keyboard('e');
    await expect.element(dialog()).toBeVisible();
  }

  function field(name: string) {
    return dialog().getByRole('textbox', { name, exact: true });
  }

  test('tab and the arrows move from row to row, round from the last to the first', async () => {
    await openForm();

    await userEvent.keyboard('{Tab}');
    await expect.element(field('Artist (Latin)')).toHaveFocus();
    await userEvent.keyboard('{ArrowDown}');
    await expect.element(field('Album')).toHaveFocus();
    await userEvent.keyboard('{ArrowUp}');
    await userEvent.keyboard('{Shift>}{Tab}{/Shift}');
    await expect.element(field('Artist')).toHaveFocus();
    await userEvent.keyboard('{ArrowUp}');
    await expect.element(field('Cut end')).toHaveFocus();
    await userEvent.keyboard('{Tab}');
    await expect.element(field('Artist')).toHaveFocus();
  });

  test('enter while a word is being composed applies nothing', async () => {
    await openForm();
    await userEvent.keyboard('Blue');

    field('Album').element().dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', isComposing: true, bubbles: true, cancelable: true }));

    expect(requestsTo(fetchMock, 'POST', '/api/bulk')).toEqual([]);
    await expect.element(dialog()).toBeVisible();
  });

});

describe('names in their own direction and ticks that are told', () => {
  const arabicCopy: CopyChoice[] = [
    { key: 'cover', label: 'Cover', has: false, text: '' },
    { key: 'artist', label: 'Artist', has: true, text: 'فيروز' },
    { key: 'album', label: 'Album', has: true, text: 'وطني' },
  ];
  const copying: StubRoute = ({ method, url }) =>
    method === 'GET' && url.pathname === '/api/tracks/sowhat/copyable' ? jsonResponse(arabicCopy) : undefined;

  test('the copy list lays out the title and every name in its own direction and says what is ticked and what is off', async () => {
    await openApp(library(), copying);
    await onRow(soWhat);

    await userEvent.keyboard('c');

    await expect.element(dialog().getByRole('option').first()).toBeVisible();
    const heading = dialog().getByRole('heading').element();
    expect(heading.querySelector('bdi[dir="auto"]')?.textContent).toBe('So What');
    for (const option of dialog().getByRole('option').elements()) {
      expect(option.querySelectorAll('bdi[dir="auto"]')).toHaveLength(2);
    }
    const [cover, artist, album] = dialog().getByRole('option').elements();
    expect(cover?.getAttribute('aria-disabled')).toBe('true');
    expect(cover?.hasAttribute('aria-checked')).toBe(false);
    expect(artist?.getAttribute('aria-checked')).toBe('true');
    await userEvent.keyboard('{ArrowDown}{ArrowDown} ');
    await expect.poll(() => album?.getAttribute('aria-checked')).toBe('false');
  });

  test('the playlist chooser and the name of a new one are laid out in their own direction and a name is at most 80 characters', async () => {
    await openApp(library([{ name: 'سهرة', count: 1, length: '0:03' }]));
    await onRow(soWhat);

    await userEvent.keyboard('p');

    await expect.element(dialog().getByRole('option').first()).toBeVisible();
    const names = dialog().getByRole('option').elements().map((option) => option.querySelector('.label bdi')?.getAttribute('dir'));
    expect(names).toEqual(['auto', 'auto']);
    await userEvent.keyboard('{ArrowDown}{Enter}');
    const name = dialog().getByRole('textbox', { name: 'Name of the playlist' });
    await expect.element(name).toBeVisible();
    expect(name.element().getAttribute('dir')).toBe('auto');
    expect(name.element().getAttribute('maxlength')).toBe('80');
  });
});
