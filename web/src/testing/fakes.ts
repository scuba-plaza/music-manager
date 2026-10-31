import type { InfoView, LibraryResponse } from '../api/types.gen';

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
