import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Profile } from '../api/types.gen';
import { changeResponse, infoView, jsonResponse, memoryStorage, requestsTo, sampleLibrary, sampleProfiles, stubServer, urlOf, type StubRoute } from '../testing/fakes';
import {
  backToList,
  blankForm,
  editFormat,
  formOf,
  leaveFormat,
  newFormat,
  openFormats,
  parseForm,
  removeFormat,
  saveFormat,
  typeFormat,
  useFormats,
} from './formats';
import { shownErrors } from './form';
import { useApp } from './store';

let fetchMock: ReturnType<typeof stubServer>;

function first(): Profile {
  const p = sampleProfiles()[0];
  if (p === undefined) {
    throw new Error('no sample profile');
  }
  return p;
}

function serve(route: StubRoute = () => undefined, stored: Profile[] = sampleProfiles()): void {
  fetchMock = stubServer(infoView(), { ...sampleLibrary(), profiles: stored }, { route });
  useApp.setState({ lib: { ...sampleLibrary(), profiles: stored } });
}

function state() {
  return useFormats.getState();
}

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
  vi.stubGlobal('localStorage', memoryStorage());
  useApp.setState(useApp.getInitialState(), true);
  useFormats.setState(useFormats.getInitialState(), true);
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe('the form of a format', () => {
  it('starts as Generic is, with no name', () => {
    expect(blankForm()).toEqual({
      name: '',
      latin_names: false,
      component_max: '200',
      stem_max: '0',
      tag_width: '0',
      latin_genre: false,
      cover_format: 'keep',
      cover_max_edge: '0',
      cover_quality: '0',
      single_covers: false,
      replaygain: true,
      playlist_ext: '.m3u8',
      playlist_style: 'relative',
    });
  });

  it('shows a stored format as text, to be edited', () => {
    const [wolfgang] = sampleProfiles() as [Profile];

    expect(formOf(wolfgang)).toMatchObject({ name: 'Wolfgang WMP01', component_max: '100', stem_max: '20', tag_width: '12', cover_max_edge: '480', cover_quality: '85', playlist_ext: '.m3u' });
  });

  it('is turned back into numbers for the server, and a number that is no number is said apart', () => {
    const { profile, local } = parseForm({ ...blankForm(), name: 'Car', component_max: '120' });

    expect(local).toEqual({});
    expect(profile).toMatchObject({ id: '', name: 'Car', component_max: 120, stem_max: 0 });

    const bad = parseForm({ ...blankForm(), name: 'Car', component_max: '12x', stem_max: '', cover_format: 'baseline-jpeg', cover_max_edge: '-4', cover_quality: '1.5' });
    expect(Object.keys(bad.local).toSorted()).toEqual(['component_max', 'cover_max_edge', 'cover_quality', 'stem_max']);
  });

  it('does not mind what is in a box that is switched off', () => {
    const { local } = parseForm({ ...blankForm(), name: 'Car', latin_names: false, tag_width: 'x', cover_format: 'keep', cover_max_edge: 'y', cover_quality: 'z' });

    expect(local).toEqual({});
  });
});

describe('the list of formats', () => {
  it('opens on Generic, which comes first', () => {
    serve();

    openFormats();

    expect(useApp.getState().overlay).toEqual({ kind: 'formats' });
    expect(state()).toMatchObject({ mode: 'list', cursor: 0 });
  });

  it('says Generic cannot be changed or deleted', () => {
    serve();
    openFormats();

    editFormat();
    removeFormat();

    expect(state().mode).toBe('list');
    expect(useApp.getState().status.text).toMatch(/Generic/);
    expect(useApp.getState().overlay).toEqual({ kind: 'formats' });
  });

  it('opens a stored format for editing, with its settings', () => {
    serve();
    openFormats();
    useFormats.setState({ cursor: 2 });

    editFormat();

    expect(state()).toMatchObject({ mode: 'form', editing: 'my-player' });
    expect(state().form).toMatchObject({ name: 'My Player', tag_width: '12' });
  });

  it('opens a new format as Generic is', () => {
    serve();
    openFormats();

    newFormat();

    expect(state()).toMatchObject({ mode: 'form', editing: null, form: blankForm() });
  });

  it('will not start a ninth format', () => {
    const many = Array.from({ length: 8 }, (_, i): Profile => ({ ...first(), id: `p${String(i)}`, name: `P${String(i)}` }));
    serve(() => undefined, many);
    openFormats();

    newFormat();

    expect(state().mode).toBe('list');
    expect(useApp.getState().status).toMatchObject({ kind: 'bad' });
    expect(useApp.getState().status.text).toMatch(/8/);
  });

  it('goes back to the list from the form', () => {
    serve();
    openFormats();
    newFormat();

    backToList();

    expect(state().mode).toBe('list');
  });
});

describe('what is typed in the form', () => {
  const check: StubRoute = ({ method, url, body }) => {
    if (method === 'POST' && url.pathname === '/api/profiles/check') {
      const p = body as Profile;
      return jsonResponse(p.name === '' ? { fields: { name: 'Name cannot be empty' } } : {});
    }
    return undefined;
  };

  it('is checked by the server once the typing pauses, and a message waits until the box is left', async () => {
    serve(check);
    openFormats();
    newFormat();

    typeFormat('component_max', '150');
    await vi.advanceTimersByTimeAsync(160);

    await vi.waitFor(() => {
      expect(state().checks.wrong).toEqual({ name: 'Name cannot be empty' });
    });
    expect(shownErrors(state().checks)).toEqual({});
    expect(requestsTo(fetchMock, 'POST', '/api/profiles/check')).toHaveLength(1);

    leaveFormat('name');

    expect(shownErrors(state().checks)).toEqual({ name: 'Name cannot be empty' });
  });

  it('sends the numbers as numbers', async () => {
    serve(check);
    openFormats();
    newFormat();

    typeFormat('name', 'Car');
    typeFormat('component_max', '150');
    await vi.advanceTimersByTimeAsync(160);

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'POST', '/api/profiles/check').at(-1)).toMatchObject({ name: 'Car', component_max: 150, replaygain: true });
    });
  });

  it('turns a box on or off with a true or a false', () => {
    serve(check);
    openFormats();
    newFormat();

    typeFormat('latin_names', true);
    typeFormat('cover_format', 'baseline-jpeg');

    expect(state().form).toMatchObject({ latin_names: true, cover_format: 'baseline-jpeg' });
  });
});

describe('saving a format', () => {
  const saved = (message: string): StubRoute => ({ method, url }) =>
    (method === 'POST' || method === 'PUT') && url.pathname.startsWith('/api/profiles') ? jsonResponse({ ...first(), name: message }) : undefined;

  it('makes a new format with POST, goes back to the list, and says so', async () => {
    serve(saved('Car'));
    openFormats();
    newFormat();
    typeFormat('name', 'Car');

    const wrong = await saveFormat();

    expect(wrong).toBeUndefined();
    expect(requestsTo(fetchMock, 'POST', '/api/profiles')).toEqual([expect.objectContaining({ id: '', name: 'Car', component_max: 200 })]);
    expect(state().mode).toBe('list');
    expect(useApp.getState().status.text).toMatch(/Car/);
    expect(useApp.getState().overlay).toEqual({ kind: 'formats' });
    expect(fetchMock.mock.calls.some(([input]) => urlOf(input).startsWith('/api/library'))).toBe(true);
  });

  it('changes a format with PUT on its id', async () => {
    serve(saved('Wolfgang'));
    openFormats();
    useFormats.setState({ cursor: 1 });
    editFormat();
    typeFormat('component_max', '90');

    await saveFormat();

    expect(requestsTo(fetchMock, 'PUT', '/api/profiles/wolfgang-wmp01')).toEqual([expect.objectContaining({ component_max: 90, name: 'Wolfgang WMP01' })]);
  });

  it('sends nothing when a number is no number, and shows it with the first wrong box named', async () => {
    serve(saved('x'));
    openFormats();
    newFormat();
    typeFormat('name', 'Car');
    typeFormat('stem_max', 'many');

    const wrong = await saveFormat();

    expect(wrong).toBe('stem_max');
    expect(requestsTo(fetchMock, 'POST', '/api/profiles')).toEqual([]);
    expect(shownErrors(state().checks)).toEqual({ stem_max: 'Must be a whole number' });
    expect(state().mode).toBe('form');
  });

  it('shows what the server found wrong, names the first box in the form, and stays', async () => {
    serve(({ method, url }) =>
      method === 'POST' && url.pathname === '/api/profiles'
        ? jsonResponse({ error: '2 fields need fixing', fields: { playlist_ext: 'Must be .m3u or .m3u8', name: 'Name cannot be empty' } }, 422)
        : undefined,
    );
    openFormats();
    newFormat();

    const wrong = await saveFormat();

    expect(wrong).toBe('name');
    expect(shownErrors(state().checks)).toEqual({ playlist_ext: 'Must be .m3u or .m3u8', name: 'Name cannot be empty' });
    expect(state()).toMatchObject({ mode: 'form', saving: false, error: '' });
  });

  it('says a refusal that is about no box in the form itself', async () => {
    serve(({ method, url }) => (method === 'POST' && url.pathname === '/api/profiles' ? jsonResponse({ error: 'There can be at most 8 formats' }, 422) : undefined));
    openFormats();
    newFormat();
    typeFormat('name', 'Car');

    const wrong = await saveFormat();

    expect(wrong).toBeUndefined();
    expect(state()).toMatchObject({ mode: 'form', error: 'There can be at most 8 formats' });
  });
});

describe('deleting a format', () => {
  it('asks first, and deletes the format when told to', async () => {
    serve(({ method, url }) => (method === 'DELETE' && url.pathname === '/api/profiles/my-player' ? new Response(null, { status: 204 }) : undefined));
    openFormats();
    useFormats.setState({ cursor: 2 });

    removeFormat();

    const overlay = useApp.getState().overlay;
    expect(overlay).toMatchObject({ kind: 'ask', head: 'Delete the format “My Player”?' });
    expect(requestsTo(fetchMock, 'DELETE', '/api/profiles/my-player')).toEqual([]);
    if (overlay?.kind === 'ask') {
      useApp.setState({ overlay: null });
      overlay.onYes();
    }
    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'DELETE', '/api/profiles/my-player')).toEqual([null]);
    });
    await vi.waitFor(() => {
      expect(useApp.getState().overlay).toEqual({ kind: 'formats' });
    });
    expect(state()).toMatchObject({ mode: 'list', cursor: 1 });
    expect(useApp.getState().status.text).toMatch(/My Player/);
  });
});

describe('closing the dialog', () => {
  it('forgets what was typed and sends no more checks', async () => {
    serve(() => jsonResponse(changeResponse()));
    openFormats();
    newFormat();
    typeFormat('name', 'Car');

    useApp.setState({ overlay: null });
    await vi.advanceTimersByTimeAsync(400);

    expect(requestsTo(fetchMock, 'POST', '/api/profiles/check')).toEqual([]);
    openFormats();
    expect(state()).toMatchObject({ mode: 'list', editing: null, form: blankForm() });
  });
});
