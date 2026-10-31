import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { page, userEvent } from 'vitest/browser';
import { render } from 'vitest-browser-react';
import { App } from '../App';
import { setStatus } from '../state/actions';
import { useApp } from '../state/store';
import { deniedStorage, FakeEventSource, infoView, libraryResponse, stubServer } from '../testing/fakes';
import { Footer } from './Footer';
import { Header } from './Header';

function hints(): string[] {
  return page
    .getByRole('list', { name: 'Keys' })
    .getByRole('listitem')
    .elements()
    .map((item) => item.textContent.trim());
}

function libraryOf(tracks: number, needWork: number, n: number) {
  const base = libraryResponse('s1', { unsynced: { known: true, n } });
  const rows = [{ kind: 'track', depth: 0, key: 'track\u001fsowhat', label: 'So What', trackId: 'sowhat', length: '0:02' }];
  return { ...base, view: { ...base.view, rows, tracks, needWork } };
}

const themeButton = () => page.getByRole('button', { name: 'Switch theme' });

beforeEach(() => {
  localStorage.clear();
  delete document.documentElement.dataset.theme;
  useApp.setState(useApp.getInitialState(), true);
  FakeEventSource.instances = [];
});

afterEach(async () => {
  vi.unstubAllGlobals();
  await page.viewport(414, 896);
});

describe('the footer', () => {
  test('the footer shows the hints for the focused pane', async () => {
    await render(<Footer />);

    await expect.poll(hints).toEqual(['↑↓ move', '→ fields', 'space mark', 'e edit', 'd remove', 'p playlist', '/ filter', 'S sort', '? help']);

    useApp.setState({ focus: 'inspector' });

    await expect.poll(hints).toEqual(['↑↓ field', 'v paste', 'esc back', '? help']);
  });

  test('the footer shows no hints while a dialog is open', async () => {
    await render(<Footer />);
    await expect.poll(() => hints().length).toBeGreaterThan(0);

    useApp.setState({ overlay: { kind: 'help' } });

    await expect.poll(hints).toEqual([]);
  });

  test('the status line shows the message and how it went', async () => {
    await render(<Footer />);
    const status = page.getByRole('status');
    await expect.element(status).toHaveTextContent('');

    setStatus('Synced to /media/player', 'good');

    await expect.element(status).toHaveTextContent('Synced to /media/player');
    await expect.element(status).toHaveClass('good');

    setStatus('Cannot reach mm. Is it still running?', 'bad');

    await expect.element(status).toHaveClass('bad');
  });
});

describe('the header', () => {
  test('the header names the library folder and counts the tracks, those that need work and those marked', async () => {
    useApp.setState({ info: infoView({ root: '/music/library' }), lib: libraryOf(6, 1, 0), marks: new Set(['a', 'b']) });

    await render(<Header />);

    const header = page.getByRole('banner');
    await expect.element(header).toHaveTextContent('/music/library');
    await expect.element(header).toHaveTextContent('6 tracks · 1 need work · 2 marked');
    expect(document.title).toBe('mm · /music/library');
  });

  test('the summary says one track in the singular and leaves out what is zero', async () => {
    useApp.setState({ info: infoView(), lib: libraryOf(1, 0, 0) });

    await render(<Header />);

    await expect.element(page.getByText('1 track', { exact: true })).toBeInTheDocument();
  });

  test('the sync chip is hidden when nothing waits and shows the count when it does', async () => {
    useApp.setState({ info: infoView(), lib: libraryOf(6, 0, 0) });
    await render(<Header />);
    const chip = page.getByRole('button', { name: /to sync/ });
    await expect.element(chip).not.toBeInTheDocument();

    useApp.setState({ lib: libraryOf(6, 0, 3) });

    await expect.element(chip).toHaveTextContent('3 to sync · press s');
  });

  test('the theme button toggles data-theme and remembers it', async () => {
    const first = await render(<Header />);
    expect(document.documentElement.dataset.theme).toBe('dark');

    await themeButton().click();

    expect(document.documentElement.dataset.theme).toBe('light');
    expect(localStorage.getItem('mm.theme')).toBe('light');

    await first.unmount();
    delete document.documentElement.dataset.theme;
    await render(<Header />);

    expect(document.documentElement.dataset.theme).toBe('light');

    await themeButton().click();

    expect(document.documentElement.dataset.theme).toBe('dark');
    expect(localStorage.getItem('mm.theme')).toBe('dark');
  });

  test('the theme switches although the browser will not store it', async () => {
    vi.stubGlobal('localStorage', deniedStorage());

    await render(<Header />);
    expect(document.documentElement.dataset.theme).toBe('dark');

    await themeButton().click();

    expect(document.documentElement.dataset.theme).toBe('light');
  });
});

describe('the application', () => {
  beforeEach(() => {
    stubServer(infoView({ root: '/music/library' }), libraryOf(6, 1, 2));
  });

  test('it loads the library and the panes are titled', async () => {
    await page.viewport(1000, 700);
    await render(<App />);

    await expect.element(page.getByRole('banner')).toHaveTextContent('/music/library');
    await expect.element(page.getByRole('banner')).toHaveTextContent('6 tracks · 1 need work');
    await expect.element(page.getByRole('button', { name: /to sync/ })).toHaveTextContent('2 to sync · press s');
    await expect.element(page.getByRole('region', { name: 'Library' })).toHaveTextContent('Library order');
    await expect.element(page.getByRole('region', { name: 'Details' })).toHaveTextContent('Details');
    expect(FakeEventSource.instances).toHaveLength(1);
  });

  test('the arrow keys switch between the panes in a window narrower than 800px', async () => {
    await page.viewport(500, 700);
    await render(<App />);
    const tree = page.getByTestId('tree-pane');
    const fields = page.getByTestId('inspector-pane');
    await expect.element(tree).toBeVisible();
    await expect.element(fields).not.toBeVisible();

    await userEvent.keyboard('{ArrowRight}');

    await expect.element(fields).toBeVisible();
    await expect.element(tree).not.toBeVisible();

    await userEvent.keyboard('{ArrowLeft}');

    await expect.element(tree).toBeVisible();
    await expect.element(fields).not.toBeVisible();
  });

  test('both panes show in a window of 800px or more', async () => {
    await page.viewport(800, 700);
    await render(<App />);
    await expect.element(page.getByRole('treeitem')).toBeVisible();

    await expect.element(page.getByTestId('tree-pane')).toBeVisible();
    await expect.element(page.getByTestId('inspector-pane')).toBeVisible();
  });

  test('text is set in the proportional face with the Arabic faces behind it, and only .mono is monospace', async () => {
    await render(<App />);
    await expect.element(page.getByRole('treeitem')).toBeVisible();
    const mono = document.createElement('span');
    mono.className = 'mono';
    document.body.append(mono);

    expect(getComputedStyle(document.body).fontFamily).toMatch(/^system-ui, "Noto Sans Arabic", "Noto Naskh Arabic"/);
    expect(getComputedStyle(mono).fontFamily).toMatch(/monospace$/);
    mono.remove();
  });
});
