import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { page, userEvent } from 'vitest/browser';
import { render } from 'vitest-browser-react';
import type { Profile } from '../api/types.gen';
import { App } from '../App';
import { useFormats } from '../state/formats';
import { useApp } from '../state/store';
import { FakeEventSource, infoView, jsonResponse, requestsTo, sampleLibrary, sampleProfiles, stubServer, type StubRoute } from '../testing/fakes';

let fetchMock: ReturnType<typeof stubServer>;
let stored: Profile[];

function dialog() {
  return page.getByRole('dialog');
}

function hints(): string[] {
  return page
    .getByRole('list', { name: 'Keys' })
    .getByRole('listitem')
    .elements()
    .map((item) => item.textContent.trim());
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
  fetchMock = stubServer(infoView(), sampleLibrary(), { route: (request) => route(request) ?? formats(request) });
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

describe('the list of formats', () => {
  test('f opens it with Generic first, then the formats of the library, and says which keys work', async () => {
    await openList();

    const rows = dialog()
      .getByRole('option')
      .elements()
      .map((o) => o.querySelector('.label bdi')?.textContent);
    expect(rows).toEqual(['Generic', 'Wolfgang WMP01', 'My Player']);
    await expect.poll(hints).toEqual(['enter edit', 'c new', 'd delete', 'esc close']);
  });

  test('enter on Generic says it cannot be changed, and the list stays', async () => {
    await openList();

    await userEvent.keyboard('{Enter}');

    await expect.element(page.getByRole('status')).toHaveTextContent('Generic is the standard format and cannot be changed');
    await expect.element(dialog().getByRole('listbox')).toBeVisible();
  });

  test('esc closes it', async () => {
    await openList();

    await userEvent.keyboard('{Escape}');

    await expect.element(dialog()).not.toBeInTheDocument();
  });
});

describe('the form of a format', () => {
  test('c opens it with every setting, grouped, the first box focused, and esc goes back to the list', async () => {
    await openForm();

    expect(
      dialog()
        .getByRole('textbox')
        .elements()
        .map((e) => (e as HTMLInputElement).labels?.[0]?.textContent),
    ).toEqual(['Name', 'Longest name', 'Cut names at', 'Artist and album width', 'Longest side', 'JPEG quality']);
    for (const heading of ['Names', 'Tags', 'Covers', 'Playback', 'Playlists']) {
      await expect.element(dialog().getByText(heading, { exact: true })).toBeVisible();
    }
    await expect.element(field('Name')).toHaveFocus();
    await expect.poll(hints).toEqual(['enter save', 'tab next', 'esc back']);

    await userEvent.keyboard('{Escape}');

    await expect.element(dialog().getByRole('listbox')).toBeVisible();
    await expect.element(dialog()).toBeVisible();
  });

  test('a box that does not apply is switched off and says why, and comes on when it does', async () => {
    await openForm();

    await expect.element(field('Artist and album width')).toBeDisabled();
    await expect.element(dialog().getByText('Needs Latin letters only')).toBeVisible();
    await expect.element(field('Longest side')).toBeDisabled();
    await expect.element(field('JPEG quality')).toBeDisabled();
    await expect.element(dialog().getByText('Only when covers are converted').first()).toBeVisible();
    expect(dialog().getByText('Only when covers are converted').elements()).toHaveLength(2);

    await userEvent.click(field('Latin letters only'));
    await userEvent.selectOptions(field('Cover'), 'baseline-jpeg');

    await expect.element(field('Artist and album width')).toBeEnabled();
    await expect.element(field('Longest side')).toBeEnabled();
    await expect.element(field('JPEG quality')).toBeEnabled();
  });

  test('a message about a box shows once it is left, from what the server says of the whole form', async () => {
    await openForm();

    await userEvent.keyboard('{Tab}');
    await expect.poll(() => requestsTo(fetchMock, 'POST', '/api/profiles/check').length).toBeGreaterThan(0);

    await expect.element(dialog().getByRole('alert')).toHaveTextContent('Name cannot be empty');
    await expect.element(field('Name')).toBeInvalid();
    await expect.element(field('Longest name')).not.toBeInvalid();
  });

  test('a number that is no number is said under its box before anything is sent', async () => {
    await openForm();
    await userEvent.fill(field('Name'), 'Car');
    await userEvent.fill(field('Cut names at'), 'many');

    await userEvent.keyboard('{Enter}');

    await expect.element(dialog().getByRole('alert')).toHaveTextContent('Must be a whole number');
    await expect.element(field('Cut names at')).toBeInvalid();
    await expect.element(field('Cut names at')).toHaveFocus();
    expect(requestsTo(fetchMock, 'POST', '/api/profiles')).toEqual([]);
  });

  test('enter saves a new format, which then shows in the list and in the Device tab choices', async () => {
    await openForm();
    await userEvent.fill(field('Name'), 'Car');
    await userEvent.click(field('Latin letters only'));
    await userEvent.fill(field('Artist and album width'), '12');
    await userEvent.fill(field('Longest name'), '90');

    await userEvent.keyboard('{Enter}');

    await expect.element(dialog().getByRole('listbox')).toBeVisible();
    expect(requestsTo(fetchMock, 'POST', '/api/profiles')).toEqual([expect.objectContaining({ name: 'Car', latin_names: true, tag_width: 12, component_max: 90, cover_format: 'keep' })]);
    const rows = dialog()
      .getByRole('option')
      .elements()
      .map((o) => o.querySelector('.label bdi')?.textContent);
    expect(rows).toEqual(['Generic', 'Wolfgang WMP01', 'My Player', 'Car']);
    await expect.element(dialog().getByRole('option', { selected: true })).toHaveTextContent('Car');
    await expect.element(page.getByRole('status')).toHaveTextContent('Saved the format “Car”');
  });

  test('a refusal that names boxes shows each of them and goes to the first, and the form stays', async () => {
    await openForm();
    await userEvent.fill(field('Name'), 'Car');
    const wrong: StubRoute = ({ method, url }) =>
      method === 'POST' && url.pathname === '/api/profiles'
        ? jsonResponse({ error: '2 fields need fixing', fields: { playlist_ext: 'Must be .m3u or .m3u8', component_max: 'Must be a whole number from 20 to 255' } }, 422)
        : undefined;
    fetchMock = stubServer(infoView(), sampleLibrary(), { route: (request) => wrong(request) ?? formats(request) });

    await userEvent.keyboard('{Enter}');

    await expect.element(dialog().getByRole('alert').first()).toBeVisible();
    expect(
      dialog()
        .getByRole('alert')
        .elements()
        .map((e) => e.textContent.trim()),
    ).toEqual(['✗ Must be a whole number from 20 to 255', '✗ Must be .m3u or .m3u8']);
    await expect.element(field('Longest name')).toHaveFocus();
    await expect.element(field('File type')).toBeInvalid();
  });

  test('enter on a format of the library opens it filled in, and enter saves what was changed with PUT', async () => {
    await openList();
    await userEvent.keyboard('{ArrowDown}{ArrowDown}{Enter}');
    await expect.element(field('Name')).toHaveValue('My Player');
    await expect.element(field('Latin letters only')).toBeChecked();
    await expect.element(field('Artist and album width')).toHaveValue('12');

    await userEvent.fill(field('Longest name'), '150');
    await userEvent.keyboard('{Enter}');

    await expect.element(dialog().getByRole('listbox')).toBeVisible();
    expect(requestsTo(fetchMock, 'PUT', '/api/profiles/my-player')).toEqual([expect.objectContaining({ name: 'My Player', component_max: 150 })]);
  });

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

describe('deleting a format', () => {
  test('d asks first, y deletes it, and the list is back without it', async () => {
    await openList();
    await userEvent.keyboard('{ArrowDown}d');
    await expect.element(dialog()).toHaveTextContent('Delete the format “Wolfgang WMP01”?');
    expect(requestsTo(fetchMock, 'DELETE', '/api/profiles/wolfgang-wmp01')).toEqual([]);

    await userEvent.keyboard('y');

    await expect.poll(() => requestsTo(fetchMock, 'DELETE', '/api/profiles/wolfgang-wmp01')).toEqual([null]);
    await expect.element(dialog().getByRole('listbox')).toBeVisible();
    const rows = dialog()
      .getByRole('option')
      .elements()
      .map((o) => o.querySelector('.label bdi')?.textContent);
    expect(rows).toEqual(['Generic', 'My Player']);
  });

  test('d on Generic says it cannot be deleted', async () => {
    await openList();

    await userEvent.keyboard('d');

    await expect.element(page.getByRole('status')).toHaveTextContent('Generic is the standard format and cannot be deleted');
  });
});
