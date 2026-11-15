import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { page} from 'vitest/browser';
import { render } from 'vitest-browser-react';
import { App } from '../App';
import { useApp } from '../state/store';
import { deniedStorage, FakeEventSource, infoView, libraryResponse, stubServer } from '../testing/fakes';
import '../styles/tokens.css';
import '../styles/app.css';
import { Header } from './Header';

function libraryOf(tracks: number, needWork: number, n: number) {
  const base = libraryResponse('s1', { unsynced: n });
  const rows = [{ kind: 'track', depth: 0, key: 'track\u001fsowhat', label: 'So What', trackId: 'sowhat', length: '0:02' }];
  return { ...base, view: { ...base.view, rows, tracks, needWork } };
}

const startingViewport = { width: window.innerWidth, height: window.innerHeight };

const themeButton = () => page.getByRole('button', { name: 'Switch theme' });

beforeEach(() => {
  localStorage.clear();
  delete document.documentElement.dataset.theme;
  useApp.setState(useApp.getInitialState(), true);
  FakeEventSource.instances = [];
});

afterEach(async () => {
  vi.unstubAllGlobals();
  await page.viewport(startingViewport.width, startingViewport.height);
});

describe('the header', () => {

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


  test('unmounting the page takes its keys, its picture paste and its stream away', async () => {
    const mounted = await render(<App />);
    await expect.element(page.getByRole('banner')).toHaveTextContent('/music/library');
    document.body.dispatchEvent(new KeyboardEvent('keydown', { key: '?', bubbles: true }));
    await expect.poll(() => useApp.getState().overlay?.kind).toBe('help');
    useApp.setState({ overlay: null });

    await mounted.unmount();
    document.body.dispatchEvent(new KeyboardEvent('keydown', { key: '?', bubbles: true }));

    expect(useApp.getState().overlay).toBeNull();
    expect(FakeEventSource.instances.filter((stream) => !stream.closed)).toEqual([]);
  });

});
