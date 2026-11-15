import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { page, userEvent } from 'vitest/browser';
import { render } from 'vitest-browser-react';
import type { JobView, SyncData, TargetView } from '../api/types.gen';
import { App } from '../App';
import { goToRow } from '../state/edit';
import { jobEvent } from '../state/jobs';
import { useApp } from '../state/store';
import { useSync } from '../state/sync';
import { FakeEventSource, infoView, jsonResponse, requestsTo, sampleLibrary, stubServer, type StubRoute } from '../testing/fakes';

const sep = '\u001f';
const soWhat = `track${sep}sowhat`;

let fetchMock: ReturnType<typeof stubServer>;

const targets: TargetView[] = [
  { label: 'Library folder', path: '~/Music', kind: 'library' },
  { label: 'USB', path: '/media/me/USB', kind: 'drive' },
];


function job(state: string, data?: SyncData, error?: string): JobView {
  return { id: 'j1', kind: 'sync', state, ...(data === undefined ? {} : { data }), ...(error === undefined ? {} : { error }) };
}

function dialog() {
  return page.getByRole('dialog');
}

function format() {
  return dialog().getByRole('combobox', { name: 'Format' });
}

function chosenTarget(): string | undefined {
  return dialog()
    .getByRole('listbox', { name: 'Where to write' })
    .getByRole('option', { selected: true })
    .element()
    .querySelector('.label')?.textContent.trim();
}

const served: StubRoute = ({ method, url }) => {
  if (method === 'GET' && url.pathname === '/api/sync/targets') {
    return jsonResponse({ targets });
  }
  if (method === 'POST' && url.pathname === '/api/sync/plan') {
    return jsonResponse(job('running'));
  }
  if (method === 'POST' && url.pathname === '/api/sync/p1/apply') {
    return jsonResponse(job('running'));
  }
  if (method === 'DELETE' && url.pathname === '/api/jobs/j1') {
    return jsonResponse(job('stopped'));
  }
  return undefined;
};

function routes(...all: StubRoute[]): StubRoute {
  return (request) => {
    for (const route of all) {
      const answer = route(request);
      if (answer !== undefined) {
        return answer;
      }
    }
    return undefined;
  };
}

async function openApp(...extra: StubRoute[]): Promise<void> {
  fetchMock = stubServer(infoView(), sampleLibrary(), { route: routes(...extra, served) });
  await render(<App />);
  await expect.element(page.getByRole('treeitem').first()).toBeVisible();
  goToRow(soWhat);
  page.getByRole('tree').element().focus();
  await expect.element(page.getByRole('tree').getByRole('treeitem', { selected: true })).toBeVisible();
}

async function open(...extra: StubRoute[]): Promise<void> {
  await openApp(...extra);
  await userEvent.keyboard('s');
  await expect.element(dialog()).toHaveAccessibleName('Sync');
}

async function typing(...extra: StubRoute[]): Promise<void> {
  await open(...extra);
  await userEvent.keyboard('{End}{Enter}');
  await expect.element(dialog().getByRole('textbox', { name: 'Folder' })).toHaveFocus();
}

beforeEach(() => {
  localStorage.clear();
  useApp.setState(useApp.getInitialState(), true);
  useSync.setState(useSync.getInitialState(), true);
  FakeEventSource.instances = [];
});

afterEach(async () => {
  useApp.setState({ overlay: null });
  jobEvent({ id: 'j1', kind: 'sync', state: 'stopped' });
  await new Promise((done) => setTimeout(done, 80));
  vi.unstubAllGlobals();
});

describe('choosing where', () => {

  test('with the format list focused the keys are the list’s own, not the targets’', async () => {
    await open();
    await userEvent.keyboard('j');
    format().element().focus();

    await userEvent.keyboard('{ArrowDown}');
    await userEvent.keyboard('j');
    await userEvent.keyboard('f');
    await userEvent.keyboard('{Enter}');

    expect(chosenTarget()).toBe('USB');
    expect(requestsTo(fetchMock, 'POST', '/api/sync/plan')).toEqual([]);
    expect(useSync.getState().phase).toBe('choosing');
  });

});

describe('typing a folder', () => {

  test('enter in the format list does not plan the folder', async () => {
    await typing();
    await userEvent.keyboard('~/player');
    format().element().focus();

    await userEvent.keyboard('{Enter}');

    expect(requestsTo(fetchMock, 'POST', '/api/sync/plan')).toEqual([]);
    expect(useSync.getState().phase).toBe('typing');
  });

  test('f is a letter in the folder, not the format key', async () => {
    await typing();

    await userEvent.keyboard('~/f');

    await expect.element(dialog().getByRole('textbox', { name: 'Folder' })).toHaveValue('~/f');
    await expect.element(format()).toHaveValue('generic');
  });

});
