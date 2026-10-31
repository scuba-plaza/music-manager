import { vi } from 'vitest';
import type { ChangeResponse, FieldView, InfoView, LibraryResponse, PeekView, ProfileView, RowView, TrackView } from '../api/types.gen';

export function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

export function urlOf(input: Parameters<typeof fetch>[0]): string {
  if (typeof input === 'string') {
    return input;
  }
  return input instanceof URL ? input.href : input.url;
}

export function requestOf(fetchMock: { mock: { calls: Parameters<typeof fetch>[] } }, index = 0): { url: string; init: RequestInit } {
  const call = fetchMock.mock.calls[index];
  if (!call) {
    throw new Error(`fetch was called ${String(fetchMock.mock.calls.length)} times, not ${String(index + 1)}`);
  }
  const [input, init] = call;
  return { url: urlOf(input), init: init ?? {} };
}

export function headerOf(init: RequestInit, name: string): string | null {
  return new Headers(init.headers).get(name);
}

export function memoryStorage(initial: Record<string, string> = {}) {
  const data = new Map(Object.entries(initial));
  return {
    data,
    getItem: (key: string): string | null => data.get(key) ?? null,
    setItem: (key: string, value: string): void => {
      data.set(key, value);
    },
  };
}

export function deniedStorage() {
  return {
    getItem: (): never => {
      throw new Error('storage is denied');
    },
    setItem: (): never => {
      throw new Error('storage is denied');
    },
  };
}

export function infoView(overrides: Partial<InfoView> = {}): InfoView {
  return { canAdd: true, lookup: true, root: '/music', version: 'test', wolfgang: false, ...overrides };
}

export function libraryResponse(stamp = 's1', overrides: Partial<LibraryResponse> = {}): LibraryResponse {
  return {
    history: {},
    stamp,
    unsynced: { known: true, n: 0 },
    view: {
      rows: [],
      tracks: 0,
      needWork: 0,
      shown: 0,
      sort: { key: '', reverse: false, descending: false, title: 'Library order', choices: [] },
      playlists: [],
    },
    ...overrides,
  };
}

export class FakeEventSource {
  static instances: FakeEventSource[] = [];

  readonly url: string;
  closed = false;
  private readonly listeners = new Map<string, ((event: Event) => void)[]>();

  constructor(url: string) {
    this.url = url;
    FakeEventSource.instances.push(this);
  }

  addEventListener(type: string, listener: (event: Event) => void): void {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), listener]);
  }

  close(): void {
    this.closed = true;
  }

  emit(type: string, data?: string): void {
    const event = data === undefined ? new Event(type) : new MessageEvent(type, { data });
    for (const listener of this.listeners.get(type) ?? []) {
      listener(event);
    }
  }
}

const sep = '\u001f';

export function sampleRows(): RowView[] {
  return [
    { kind: 'group', depth: 0, key: 'playlists', label: 'Playlists', count: 1 },
    { kind: 'playlist', depth: 1, key: `playlist${sep}Late night`, parent: 'playlists', label: 'Late night', count: 1, editable: true },
    {
      kind: 'track',
      depth: 2,
      key: `pltrack${sep}Late night${sep}blue`,
      parent: `playlist${sep}Late night`,
      label: 'Blue in Green',
      detail: 'Miles Davis',
      num: '01',
      trackId: 'blue',
      length: '0:03',
    },
    { kind: 'group', depth: 0, key: 'needs', label: 'Needs work', count: 1 },
    { kind: 'track', depth: 1, key: `track${sep}rough`, parent: 'needs', label: 'Rough Mix', detail: 'Basement Tapes', trackId: 'rough', length: '0:05' },
    { kind: 'artist', depth: 0, key: `artist${sep}فيروز`, label: 'فيروز', artist: 'فيروز', count: 1, editable: true },
    {
      kind: 'album',
      depth: 1,
      key: `album${sep}فيروز${sep}Songs`,
      parent: `artist${sep}فيروز`,
      label: 'Songs',
      artist: 'فيروز',
      album: 'Songs',
      count: 1,
      editable: true,
    },
    { kind: 'track', depth: 2, key: `track${sep}watani`, parent: `album${sep}فيروز${sep}Songs`, label: 'وطني', num: '01', trackId: 'watani', length: '0:04' },
    { kind: 'artist', depth: 0, key: `artist${sep}John Coltrane`, label: 'John Coltrane', artist: 'John Coltrane', count: 1, editable: true },
    {
      kind: 'album',
      depth: 1,
      key: `album${sep}John Coltrane${sep}Giant Steps`,
      parent: `artist${sep}John Coltrane`,
      label: 'Giant Steps',
      artist: 'John Coltrane',
      album: 'Giant Steps',
      count: 1,
      editable: true,
    },
    {
      kind: 'track',
      depth: 2,
      key: `track${sep}giant`,
      parent: `album${sep}John Coltrane${sep}Giant Steps`,
      label: 'Giant Steps',
      num: '01',
      trackId: 'giant',
      length: '0:03',
    },
    { kind: 'artist', depth: 0, key: `artist${sep}Miles Davis`, label: 'Miles Davis', artist: 'Miles Davis', count: 2, editable: true },
    {
      kind: 'album',
      depth: 1,
      key: `album${sep}Miles Davis${sep}Kind of Blue`,
      parent: `artist${sep}Miles Davis`,
      label: 'Kind of Blue',
      artist: 'Miles Davis',
      album: 'Kind of Blue',
      count: 2,
      editable: true,
    },
    {
      kind: 'track',
      depth: 2,
      key: `track${sep}sowhat`,
      parent: `album${sep}Miles Davis${sep}Kind of Blue`,
      label: 'So What',
      num: '01',
      trackId: 'sowhat',
      length: '0:02',
    },
    {
      kind: 'track',
      depth: 2,
      key: `track${sep}blue`,
      parent: `album${sep}Miles Davis${sep}Kind of Blue`,
      label: 'Blue in Green',
      num: '03',
      trackId: 'blue',
      length: '0:03',
    },
    { kind: 'artist', depth: 0, key: `artist${sep}Nina Simone`, label: 'Nina Simone', artist: 'Nina Simone', count: 1, editable: true },
    { kind: 'album', depth: 1, key: `singles${sep}Nina Simone`, parent: `artist${sep}Nina Simone`, label: 'Singles', artist: 'Nina Simone', count: 1 },
    { kind: 'track', depth: 2, key: `track${sep}good`, parent: `singles${sep}Nina Simone`, label: 'Feeling Good', trackId: 'good', length: '0:04' },
  ];
}

export function sampleLibrary(stamp = 's1'): LibraryResponse {
  const base = libraryResponse(stamp);
  const rows = sampleRows();
  return {
    ...base,
    view: { ...base.view, rows, tracks: 6, needWork: 1, shown: rows.filter((row) => row.trackId !== undefined).length },
  };
}

function chainOf(byKey: ReadonlyMap<string, RowView>, row: RowView): RowView[] {
  const chain = [row];
  for (let parent = byKey.get(row.parent ?? ''); parent; parent = byKey.get(parent.parent ?? '')) {
    chain.push(parent);
  }
  return chain;
}

export function narrowRows(rows: readonly RowView[], filter: string): RowView[] {
  const words = filter.split(/\s+/).filter((word) => word !== '');
  if (words.length === 0) {
    return [...rows];
  }
  const byKey = new Map(rows.map((row) => [row.key, row]));
  const kept = new Set<string>();
  for (const row of rows.filter((candidate) => candidate.kind === 'track')) {
    const chain = chainOf(byKey, row);
    const text = chain
      .map((link) => `${link.label} ${link.detail ?? ''}`)
      .join(' ')
      .toLowerCase();
    const matches = words.every((word) =>
      word === 'status:unidentified' ? chain.some((link) => link.key === 'needs') : text.includes(word.toLowerCase()),
    );
    if (matches) {
      for (const link of chain) {
        kept.add(link.key);
      }
    }
  }
  return rows.filter((row) => kept.has(row.key));
}

export function trackViewFor(row: RowView): TrackView {
  return {
    id: row.trackId ?? '',
    title: row.label,
    identified: true,
    tabs: [{ id: 'details', name: 'Details', fields: [] }],
    path: `${row.label}.mp3`,
    dates: '',
    about: '',
    reveal: [],
  };
}

export function peekViewFor(row: RowView): PeekView {
  return { kind: row.kind, title: row.kind, label: row.label, editable: row.editable === true, info: [], list: [] };
}


export function sampleProfiles(): ProfileView[] {
  return [
    { name: 'generic', label: 'Generic', builtin: true },
    { name: 'wolfgang-wmp01', label: 'Wolfgang WMP01', builtin: true },
    { name: 'my-player', label: 'My Player', builtin: false },
  ];
}

export function fieldView(key: string, overrides: Partial<FieldView> = {}): FieldView {
  return { key, label: key, text: '', empty: '—', editable: true, edit: '', ...overrides };
}

export function changeResponse(overrides: Partial<ChangeResponse> = {}): ChangeResponse {
  return { history: {}, ...overrides };
}

export function withDevice(view: TrackView, profile: string): TrackView {
  const device = {
    id: 'device',
    name: 'Device',
    fields: [],
    device: {
      rows: [
        { label: 'Folder', value: `${profile}/Folder` },
        { label: 'File', value: `01 - ${view.title}.mp3` },
      ],
      warnings: [`a warning for ${profile}`],
      footnote: 'A preview: nothing of it is stored.',
    },
  };
  return { ...view, tabs: [...view.tabs, device] };
}

export interface StubRequest {
  method: string;
  url: URL;
  body: unknown;
}

export type StubRoute = (request: StubRequest) => Response | undefined;

export interface StubOptions {
  route?: StubRoute;
  profiles?: ProfileView[];
}

function bodyOf(init: RequestInit | undefined): unknown {
  return typeof init?.body === 'string' ? JSON.parse(init.body) : null;
}

function answer(request: StubRequest, info: InfoView, library: LibraryResponse, options: StubOptions): Response {
  const routed = options.route?.(request);
  if (routed !== undefined) {
    return routed;
  }
  const target = request.url;
  const rows = library.view.rows;
  const reading = request.method === 'GET';
  if (reading && target.pathname === '/api/info') {
    return jsonResponse(info);
  }
  if (reading && target.pathname === '/api/profiles') {
    return jsonResponse(options.profiles ?? sampleProfiles());
  }
  if (reading && target.pathname === '/api/library') {
    const narrowed = narrowRows(rows, target.searchParams.get('q') ?? '');
    const shown = narrowed.filter((row) => row.trackId !== undefined).length;
    return jsonResponse({ ...library, view: { ...library.view, rows: narrowed, shown } });
  }
  const track = /^\/api\/tracks\/([^/]+)$/.exec(target.pathname);
  if (reading && track) {
    const found = rows.find((row) => row.trackId === decodeURIComponent(track[1] ?? ''));
    const profile = target.searchParams.get('profile');
    if (profile !== null && !(options.profiles ?? sampleProfiles()).some((p) => p.name === profile)) {
      return jsonResponse({ error: `there is no device profile named "${profile}"` }, 400);
    }
    if (!found) {
      return jsonResponse({ error: 'not found' }, 404);
    }
    return jsonResponse(profile === null ? trackViewFor(found) : withDevice(trackViewFor(found), profile));
  }
  if (reading && target.pathname === '/api/peek') {
    const found = rows.find((row) => row.key === target.searchParams.get('key'));
    return found ? jsonResponse(peekViewFor(found)) : jsonResponse({ error: 'not found' }, 404);
  }
  return jsonResponse({ error: `the stub server has no answer for ${request.method} ${target.pathname}` }, 404);
}

export function stubServer(info: InfoView = infoView(), library: LibraryResponse = libraryResponse(), options: StubOptions = {}) {
  const fetchMock = vi.fn<typeof fetch>((input, init) => {
    const request = { method: init?.method ?? 'GET', url: new URL(urlOf(input), 'http://stub.invalid'), body: bodyOf(init) };
    return Promise.resolve(answer(request, info, library, options));
  });
  vi.stubGlobal('fetch', fetchMock);
  vi.stubGlobal('EventSource', FakeEventSource);
  return fetchMock;
}

export function requestsTo(fetchMock: { mock: { calls: Parameters<typeof fetch>[] } }, method: string, path: string): unknown[] {
  return fetchMock.mock.calls.flatMap(([input, init]) => {
    const url = new URL(urlOf(input), 'http://stub.invalid');
    const sent = init?.method ?? 'GET';
    return sent === method && url.pathname + url.search === path ? [bodyOf(init)] : [];
  });
}
