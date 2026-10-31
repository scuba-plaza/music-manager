import type { ErrorView } from './types.gen';

const unreachable = 'Cannot reach mm. Is it still running?';

export class ApiError extends Error {
  readonly status: number;

  constructor(message: string, status = 0) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

type Method = 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE';

function isAbort(err: unknown): boolean {
  return err instanceof Error && err.name === 'AbortError';
}

function isErrorBody(data: unknown): data is ErrorView {
  return typeof data === 'object' && data !== null && 'error' in data && typeof data.error === 'string' && data.error !== '';
}

async function readBody(res: Response): Promise<unknown> {
  const type = res.headers.get('Content-Type') ?? '';
  if (!type.includes('json')) {
    return null;
  }
  const data: unknown = await res.json();
  return data;
}

async function errorBody(res: Response): Promise<unknown> {
  try {
    return await readBody(res);
  } catch {
    return null;
  }
}

async function failure(res: Response): Promise<ApiError> {
  const data = await errorBody(res);
  if (isErrorBody(data)) {
    return new ApiError(data.error, res.status);
  }
  const text = res.statusText === '' ? `Request failed (${String(res.status)})` : `${String(res.status)} ${res.statusText}`;
  return new ApiError(text, res.status);
}

async function call<T>(method: Method, path: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  const headers: Record<string, string> = { 'X-MM': '1' };
  const init: RequestInit = { method, headers };
  if (signal) {
    init.signal = signal;
  }
  if (body instanceof FormData || body instanceof Blob) {
    init.body = body;
  } else if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
    init.body = JSON.stringify(body);
  }
  let res: Response;
  try {
    res = await fetch(path, init);
  } catch (err) {
    if (isAbort(err)) {
      throw err;
    }
    throw new ApiError(unreachable);
  }
  if (!res.ok) {
    throw await failure(res);
  }
  return (await readBody(res)) as T;
}

export function get<T>(path: string, init?: { signal?: AbortSignal }): Promise<T> {
  return call<T>('GET', path, undefined, init?.signal);
}

export function send<T>(method: 'POST' | 'PATCH' | 'PUT' | 'DELETE', path: string, body?: unknown): Promise<T> {
  return call<T>(method, path, body);
}

export function query(params: Record<string, string | number | boolean | null | undefined>): string {
  const q = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === false || value === '') {
      continue;
    }
    q.set(key, value === true ? '1' : String(value));
  }
  const text = q.toString();
  return text === '' ? '' : `?${text}`;
}
