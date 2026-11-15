import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { page, userEvent } from 'vitest/browser';
import { render } from 'vitest-browser-react';
import type { Profile } from '../api/types.gen';
import { App } from '../App';
import { useFormats } from '../state/formats';
import { useApp } from '../state/store';
import { FakeEventSource, infoView, jsonResponse, sampleLibrary, sampleProfiles, stubServer, type StubRoute } from '../testing/fakes';

let stored: Profile[];

function dialog() {
  return page.getByRole('dialog');
}

function field(name: string) {
  return dialog().getByLabelText(name, { exact: true });
}

function library(): Response {
  return jsonResponse({ ...sampleLibrary(), profiles: stored });
}

const formats: StubRoute = ({ method, url, body }) => {
  if (method === 'GET' && url.pathname === '/api/library') {
    return library();
  }
  if (method === 'POST' && url.pathname === '/api/profiles/check') {
    const p = body as Profile;
    return jsonResponse(p.name.trim() === '' ? { fields: { name: 'Name cannot be empty' } } : {});
  }
  if (method === 'POST' && url.pathname === '/api/profiles') {
    const made = { ...(body as Profile), id: 'car' };
    stored = [...stored, made];
    return jsonResponse(made);
  }
  const one = /^\/api\/profiles\/([^/]+)$/.exec(url.pathname);
  if (method === 'PUT' && one) {
    const changed = { ...(body as Profile), id: one[1] ?? '' };
    stored = stored.map((p) => (p.id === changed.id ? changed : p));
    return jsonResponse(changed);
  }
  if (method === 'DELETE' && one) {
    stored = stored.filter((p) => p.id !== one[1]);
    return new Response(null, { status: 204 });
  }
  return undefined;
};

async function openApp(route: StubRoute = formats): Promise<void> {
  stubServer(infoView(), sampleLibrary(), { route: (request) => route(request) ?? formats(request) });
  await render(<App />);
  await expect.element(page.getByRole('treeitem').first()).toBeVisible();
}

async function openList(route?: StubRoute): Promise<void> {
  await openApp(route);
  await userEvent.keyboard('f');
  await expect.element(dialog()).toHaveAccessibleName('Formats');
}

async function openForm(): Promise<void> {
  await openList();
  await userEvent.keyboard('c');
  await expect.element(field('Name')).toBeVisible();
}

beforeEach(() => {
  stored = sampleProfiles();
  useApp.setState(useApp.getInitialState(), true);
  useFormats.setState(useFormats.getInitialState(), true);
  FakeEventSource.instances = [];
  vi.stubGlobal('EventSource', FakeEventSource);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('the form of a format', () => {

  test('q in a text box is a letter, and in a box that is not text it goes back like esc', async () => {
    await openForm();

    await userEvent.fill(field('Name'), 'Quiet');
    await expect.element(field('Name')).toHaveValue('Quiet');
    await expect.element(dialog()).toBeVisible();

    field('Latin letters only').element().focus();
    await userEvent.keyboard('q');

    await expect.element(dialog().getByRole('listbox')).toBeVisible();
  });
});
