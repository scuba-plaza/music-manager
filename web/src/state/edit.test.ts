import { afterEach, beforeEach, describe, expect, it, vi, type MockInstance } from 'vitest';
import type { PeekView, TrackView } from '../api/types.gen';
import {
  changeResponse,
  fieldView,
  infoView,
  jsonResponse,
  memoryStorage,
  requestsTo,
  sampleLibrary,
  stubServer,
  type StubRoute,
} from '../testing/fakes';
import { loadDetail, run } from './actions';
import { applyChange, cancelEdit, chooseField, saveEdit, startEdit, stopPlaying, typeEdit } from './edit';
import { chooseProfile, loadProfiles } from './profiles';
import { useApp } from './store';

vi.mock('./treeDom', () => ({
  pageSize: vi.fn(() => 3),
  centreCursor: vi.fn(),
  focusFilterBox: vi.fn(),
  leaveFilterBox: vi.fn(),
}));

const sep = '\u001f';
const keys = {
  miles: `artist${sep}Miles Davis`,
  kindOfBlue: `album${sep}Miles Davis${sep}Kind of Blue`,
  soWhat: `track${sep}sowhat`,
  blue: `track${sep}blue`,
};

function soWhat(overrides: Partial<TrackView> = {}): TrackView {
  return {
    id: 'sowhat',
    title: 'So What',
    identified: true,
    coverSha: 'abc123',
    tabs: [
      {
        id: 'details',
        name: 'Details',
        fields: [
          fieldView('title', { label: 'Title', text: 'So What', edit: 'So What' }),
          fieldView('track', { label: 'Track', text: '1 of 5', edit: '1', second: '5' }),
          fieldView('genre', { label: 'Genre', text: 'Jazz', edit: 'Jazz', dim: true }),
          fieldView('cover', { label: 'Cover', text: '500×500 JPEG', empty: 'none' }),
        ],
      },
      {
        id: 'audio',
        name: 'Audio',
        fields: [fieldView('cutStart', { label: 'Cut start', text: '0:00' }), fieldView('volume', { label: 'Volume', text: '0 dB' })],
      },
      {
        id: 'latin',
        name: 'Latin',
        fields: [fieldView('latinGenre', { label: 'Genre', editable: false, dim: true })],
      },
    ],
    path: 'Miles Davis/Kind of Blue/01 - So What.mp3',
    dates: '',
    about: '',
    reveal: [],
    ...overrides,
  };
}

function album(overrides: Partial<PeekView> = {}): PeekView {
  return {
    kind: 'album',
    title: 'Album',
    label: 'Kind of Blue',
    editable: true,
    info: [],
    fields: [
      fieldView('album', { label: 'Name', text: 'Kind of Blue', edit: 'Kind of Blue' }),
      fieldView('genre', { label: 'Genre', text: '', edit: '', mixed: true, empty: '(mixed)' }),
      fieldView('cover', { label: 'Cover', text: 'the same on all 2 tracks', empty: 'none' }),
    ],
    coverTrack: 'sowhat',
    coverSha: 'abc123',
    list: [],
    ...overrides,
  };
}

let fetchMock: ReturnType<typeof stubServer>;

function serve(route: StubRoute = () => undefined): void {
  fetchMock = stubServer(infoView(), sampleLibrary(), { route });
}

function onTrack(extra: Partial<ReturnType<typeof useApp.getState>> = {}): void {
  useApp.setState({ lib: sampleLibrary(), cursor: keys.soWhat, detail: soWhat(), focus: 'inspector', expanded: new Set([keys.miles, keys.kindOfBlue]), ...extra });
}

function onAlbum(extra: Partial<ReturnType<typeof useApp.getState>> = {}): void {
  useApp.setState({ lib: sampleLibrary(), cursor: keys.kindOfBlue, detail: album(), focus: 'inspector', expanded: new Set([keys.miles]), ...extra });
}

beforeEach(() => {
  vi.stubGlobal('localStorage', memoryStorage());
  useApp.setState(useApp.getInitialState(), true);
  serve();
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('typing in a field', () => {
  it('startEdit opens the field under the cursor with what it holds', () => {
    onTrack();

    run('startEdit');

    expect(useApp.getState().edit).toEqual({ field: 0, text: 'So What', second: '' });
  });

  it('the number of a track opens with the number of tracks beside it', () => {
    onTrack({ field: 1 });

    startEdit();

    expect(useApp.getState().edit).toEqual({ field: 1, text: '1', second: '5' });
  });

  it('a field with no name to write in Latin letters does not open and says so', () => {
    onTrack({ tab: 2 });

    startEdit();

    expect(useApp.getState().edit).toBeNull();
    expect(useApp.getState().status).toEqual({ text: 'There is no genre to write in Latin letters', kind: '' });
  });

  it('a mixed field opens empty', () => {
    onAlbum({ field: 1 });

    startEdit();

    expect(useApp.getState().edit).toMatchObject({ field: 1, text: '' });
  });

  it('startEdit does nothing in the tree', () => {
    onTrack({ focus: 'tree' });

    startEdit();

    expect(useApp.getState().edit).toBeNull();
  });

  it('typing changes the text and wipes the error of the last save', () => {
    onTrack({ edit: { field: 0, text: 'So What', error: 'refused' } });

    typeEdit('text', 'So When');
    typeEdit('second', '7');

    expect(useApp.getState().edit).toMatchObject({ text: 'So When', second: '7', error: '' });
  });

  it('cancelEdit closes the field and sends nothing', () => {
    onTrack({ edit: { field: 0, text: 'So When' } });

    cancelEdit();

    expect(useApp.getState().edit).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('clicking a field chooses it, and clicking it again edits it', () => {
    onTrack({ focus: 'tree' });

    chooseField(1);
    expect(useApp.getState()).toMatchObject({ focus: 'inspector', field: 1, edit: null });

    chooseField(1);
    expect(useApp.getState().edit).toMatchObject({ field: 1, text: '1' });
  });
});

describe('saving a field', () => {
  it('sends the field, the text and the second number of a track, then shows the library again', async () => {
    serve(({ method, url }) =>
      method === 'PATCH' && url.pathname === '/api/tracks/sowhat' ? jsonResponse(changeResponse({ message: 'Saved', reveal: 'sowhat' })) : undefined,
    );
    onTrack({ field: 1, edit: { field: 1, text: '2', second: '5' } });

    await saveEdit();

    expect(requestsTo(fetchMock, 'PATCH', '/api/tracks/sowhat')).toEqual([{ field: 'track', text: '2', second: '5' }]);
    expect(requestsTo(fetchMock, 'GET', '/api/library')).toHaveLength(1);
    expect(useApp.getState()).toMatchObject({ edit: null, busy: false, status: { text: 'Saved', kind: '' } });
  });

  it('a field of an album is sent with the key of the album', async () => {
    serve(({ method, url }) => (method === 'PATCH' && url.pathname === '/api/group' ? jsonResponse(changeResponse({ message: 'Saved' })) : undefined));
    onAlbum({ edit: { field: 0, text: 'Kind of Blues' } });

    await saveEdit();

    expect(requestsTo(fetchMock, 'PATCH', '/api/group')).toEqual([{ key: keys.kindOfBlue, field: 'album', text: 'Kind of Blues' }]);
    expect(useApp.getState().edit).toBeNull();
  });

  it('a refusal stays under the field, which stays open', async () => {
    serve(({ method }) => (method === 'PATCH' ? jsonResponse({ error: 'The title cannot be empty' }, 422) : undefined));
    onTrack({ edit: { field: 0, text: '' } });

    await saveEdit();

    expect(useApp.getState().edit).toEqual({ field: 0, text: '', error: 'The title cannot be empty' });
    expect(useApp.getState().busy).toBe(false);
  });

  it('saving what was there sends nothing and closes the field', async () => {
    onTrack({ edit: { field: 0, text: 'So What', second: '' } });

    await saveEdit();

    expect(fetchMock).not.toHaveBeenCalled();
    expect(useApp.getState().edit).toBeNull();
  });

  it('saving a dim value of a track as it was sets it by hand', async () => {
    serve(({ method }) => (method === 'PATCH' ? jsonResponse(changeResponse({ same: true })) : undefined));
    onTrack({ field: 2, edit: { field: 2, text: 'Jazz' } });

    await saveEdit();

    expect(requestsTo(fetchMock, 'PATCH', '/api/tracks/sowhat')).toEqual([{ field: 'genre', text: 'Jazz', second: '' }]);
  });

  it('a mixed field left empty keeps every value: nothing is sent', async () => {
    onAlbum({ field: 1, edit: { field: 1, text: '' } });

    await saveEdit();

    expect(fetchMock).not.toHaveBeenCalled();
    expect(useApp.getState().edit).toBeNull();
  });

  it('- in a mixed field is sent, to clear it on every track', async () => {
    serve(({ method }) => (method === 'PATCH' ? jsonResponse(changeResponse({ message: 'Cleared' })) : undefined));
    onAlbum({ field: 1, edit: { field: 1, text: '-' } });

    await saveEdit();

    expect(requestsTo(fetchMock, 'PATCH', '/api/group')).toEqual([{ key: keys.kindOfBlue, field: 'genre', text: '-' }]);
  });

  it('a change the server calls the same does not read the library again', async () => {
    await applyChange(() => Promise.resolve(changeResponse({ same: true })));

    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('a change follows the moved rows and reveals the track', async () => {
    onTrack({ expanded: new Set() });

    await applyChange(() => Promise.resolve(changeResponse({ message: 'Moved', reveal: 'blue' })));

    expect(useApp.getState().cursor).toBe(keys.blue);
    expect([...useApp.getState().expanded]).toEqual(expect.arrayContaining([keys.miles, keys.kindOfBlue]));
  });
});

describe('the note while typing', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('says where the file of a track would go', async () => {
    serve(({ method, url }) =>
      method === 'POST' && url.pathname === '/api/tracks/sowhat/path' ? jsonResponse({ path: 'Miles Davis/Kind of Blue/01 - So When.mp3' }) : undefined,
    );
    onTrack();
    startEdit();

    typeEdit('text', 'So When');
    await vi.advanceTimersByTimeAsync(150);

    expect(requestsTo(fetchMock, 'POST', '/api/tracks/sowhat/path').at(-1)).toEqual({ field: 'title', text: 'So When', second: '' });
    expect(useApp.getState().edit?.note).toBe('→ Miles Davis/Kind of Blue/01 - So When.mp3');
  });

  it('says which album a new name of an album would join', async () => {
    serve(({ method, url }) => (method === 'POST' && url.pathname === '/api/group/hint' ? jsonResponse({ hint: 'joins the album Blue (3 tracks)' }) : undefined));
    onAlbum();
    startEdit();

    typeEdit('text', 'Blue');
    await vi.advanceTimersByTimeAsync(150);

    expect(requestsTo(fetchMock, 'POST', '/api/group/hint').at(-1)).toEqual({ key: keys.kindOfBlue, field: 'album', text: 'Blue' });
    expect(useApp.getState().edit?.note).toBe('joins the album Blue (3 tracks)');
  });
});

describe('cuts, gains and fades', () => {
  function onCut(extra: Partial<ReturnType<typeof useApp.getState>> = {}): void {
    onTrack({ tab: 1, ...extra });
  }

  beforeEach(() => {
    serve(({ method, url }) => {
      if (method === 'GET' && url.pathname === '/api/tracks/sowhat') {
        return jsonResponse(soWhat());
      }
      return method === 'POST' && /\/(nudge|clear)$/.test(url.pathname) ? jsonResponse(changeResponse({ message: 'Changed' })) : undefined;
    });
  });

  it('[ ] { } nudge the field by a step or a big step', async () => {
    onCut();

    run('nudgeDown');
    run('nudgeUpMore');
    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'POST', '/api/tracks/sowhat/nudge')).toHaveLength(1);
    });
    expect(requestsTo(fetchMock, 'POST', '/api/tracks/sowhat/nudge')).toEqual([{ field: 'cutStart', more: -1, big: false }]);
    await vi.waitFor(() => {
      expect(useApp.getState().busy).toBe(false);
    });

    run('nudgeUpMore');
    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'POST', '/api/tracks/sowhat/nudge')).toHaveLength(2);
    });
    expect(requestsTo(fetchMock, 'POST', '/api/tracks/sowhat/nudge')[1]).toEqual({ field: 'cutStart', more: 1, big: true });
  });

  it('0 clears the field', async () => {
    onCut({ field: 1 });

    run('clearField');

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'POST', '/api/tracks/sowhat/clear')).toEqual([{ field: 'volume' }]);
    });
  });

  it('the nudge keys and 0 leave a field that is not a cut, a gain or a fade alone', async () => {
    onTrack();

    run('nudgeUp');
    run('clearField');
    await Promise.resolve();

    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('a refused nudge is said in the status line', async () => {
    serve(({ method }) => (method === 'POST' ? jsonResponse({ error: 'the cut cannot pass the end' }, 422) : undefined));
    onCut();

    run('nudgeUp');

    await vi.waitFor(() => {
      expect(useApp.getState().status).toEqual({ text: 'the cut cannot pass the end', kind: 'bad' });
    });
  });
});

describe('hearing a cut', () => {
  let created: MockInstance<(obj: Blob | MediaSource) => string>;
  let revoked: MockInstance<(url: string) => void>;

  function servePreview(): void {
    serve(({ method, url }) =>
      method === 'GET' && url.pathname === '/api/tracks/sowhat/preview'
        ? new Response(new Blob([new Uint8Array([0xff, 0xfb])], { type: 'audio/mpeg' }), { headers: { 'Content-Type': 'audio/mpeg' } })
        : undefined,
    );
  }

  beforeEach(() => {
    servePreview();
    created = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:preview');
    revoked = vi.spyOn(URL, 'revokeObjectURL').mockReturnValue(undefined);
  });

  it('p fetches the 8 s around the cut and plays it', async () => {
    onTrack({ tab: 1 });

    run('hear');
    expect(useApp.getState().playing).toEqual({ id: 'sowhat', field: 'cutStart' });

    await vi.waitFor(() => {
      expect(useApp.getState().playing).toEqual({ id: 'sowhat', field: 'cutStart', src: 'blob:preview' });
    });
    expect(requestsTo(fetchMock, 'GET', '/api/tracks/sowhat/preview?field=cutStart')).toHaveLength(1);
    expect(useApp.getState().status.text).toBe('Playing · p or esc stops');
  });

  it('p again stops it and lets the audio go', async () => {
    onTrack({ tab: 1 });
    run('hear');
    await vi.waitFor(() => {
      expect(useApp.getState().playing?.src).toBe('blob:preview');
    });

    run('hear');

    expect(useApp.getState().playing).toBeNull();
    expect(revoked).toHaveBeenCalledWith('blob:preview');
  });

  it('esc stops it and stays in the fields', async () => {
    onTrack({ tab: 1 });
    run('hear');
    await vi.waitFor(() => {
      expect(useApp.getState().playing?.src).toBe('blob:preview');
    });

    run('escape');

    expect(useApp.getState()).toMatchObject({ playing: null, focus: 'inspector' });
  });

  it('moving the cursor stops it', async () => {
    onTrack({ tab: 1 });
    run('hear');
    await vi.waitFor(() => {
      expect(useApp.getState().playing?.src).toBe('blob:preview');
    });

    useApp.setState({ focus: 'tree' });
    run('moveDown');

    expect(useApp.getState().playing).toBeNull();
  });

  it('a preview stopped while it was being cut is not played', async () => {
    onTrack({ tab: 1 });
    run('hear');

    expect(stopPlaying()).toBe(true);
    await new Promise((resolve) => setTimeout(resolve, 20));

    expect(useApp.getState().playing).toBeNull();
    expect(created).not.toHaveBeenCalled();
  });

  it('p leaves a field that is not a cut alone', () => {
    onTrack();

    run('hear');

    expect(useApp.getState().playing).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('a preview that cannot be cut is said in the status line', async () => {
    serve(({ url }) => (url.pathname.endsWith('/preview') ? jsonResponse({ error: 'ffmpeg is not installed' }, 500) : undefined));
    onTrack({ tab: 1 });

    run('hear');

    await vi.waitFor(() => {
      expect(useApp.getState().status).toEqual({ text: 'ffmpeg is not installed', kind: 'bad' });
    });
    expect(useApp.getState().playing).toBeNull();
  });
});

describe('removing a cover', () => {
  beforeEach(() => {
    serve(({ method, url }) => (method === 'DELETE' && url.pathname === '/api/cover' ? jsonResponse(changeResponse({ message: 'Cover removed' })) : undefined));
  });

  it('x on the Cover field of a track removes its cover at once', async () => {
    onTrack({ field: 3 });

    run('removeCover');

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'DELETE', '/api/cover?ids=sowhat')).toHaveLength(1);
    });
  });

  it('x on the Cover field of an album removes the covers of its tracks', async () => {
    onAlbum({ field: 2 });

    run('removeCover');

    await vi.waitFor(() => {
      expect(requestsTo(fetchMock, 'DELETE', `/api/cover?${new URLSearchParams({ group: keys.kindOfBlue }).toString()}`)).toHaveLength(1);
    });
  });

  it('x says so when there is no cover to remove', () => {
    onTrack({ field: 3, detail: soWhat({ coverSha: '' }) });

    run('removeCover');

    expect(useApp.getState().status.text).toBe('There is no cover to remove');
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('x leaves any other field alone', () => {
    onTrack({ field: 0 });

    run('removeCover');

    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe('the details of what the cursor is on', () => {
  it('a track is asked for with the device profile chosen', async () => {
    onTrack({ profile: 'my-player' });

    await loadDetail();

    expect(requestsTo(fetchMock, 'GET', '/api/tracks/sowhat?profile=my-player')).toHaveLength(1);
  });

  it('details that cannot be had are said in the pane and in the status line, until other details come', async () => {
    serve(({ url }) => (url.pathname === '/api/tracks/sowhat' ? jsonResponse({ error: 'the track is gone' }, 404) : undefined));
    onTrack();

    await loadDetail();
    expect(useApp.getState()).toMatchObject({ detail: null, detailError: 'the track is gone', status: { text: 'the track is gone', kind: 'bad' } });

    serve();
    await loadDetail();
    expect(useApp.getState().detailError).toBe('');
  });
});

describe('the device profiles', () => {
  it('the first profile that is not the generic one is chosen at first', async () => {
    await loadProfiles();

    expect(useApp.getState().profile).toBe('wolfgang-wmp01');
    expect(useApp.getState().profiles.map((p) => p.name)).toEqual(['generic', 'wolfgang-wmp01', 'my-player']);
  });

  it('a profile chosen before is kept', async () => {
    useApp.setState({ profile: 'my-player' });

    await loadProfiles();

    expect(useApp.getState().profile).toBe('my-player');
  });

  it('the details of a track shown are read again with the profile', async () => {
    onTrack();

    await loadProfiles();

    expect(requestsTo(fetchMock, 'GET', '/api/tracks/sowhat?profile=wolfgang-wmp01')).toHaveLength(1);
  });

  it('choosing a profile reads the details again with it', async () => {
    onTrack({ profile: 'wolfgang-wmp01' });

    await chooseProfile('my-player');

    expect(useApp.getState().profile).toBe('my-player');
    expect(requestsTo(fetchMock, 'GET', '/api/tracks/sowhat?profile=my-player')).toHaveLength(1);
  });
});
