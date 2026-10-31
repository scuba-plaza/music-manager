import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { headerOf, jsonResponse, requestOf } from '../testing/fakes';
import { ApiError, get, getBlob, query, send } from './client';

const fetchMock = vi.fn<typeof fetch>();

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('the client', () => {
  it('sends X-MM on every request', async () => {
    fetchMock.mockImplementation(() => Promise.resolve(jsonResponse({})));

    await get('/api/info');
    await send('POST', '/api/undo');
    await send('PATCH', '/api/tracks/a', { field: 'title', text: 'x' });
    await send('PUT', '/api/cover', { id: 'a' });
    await send('DELETE', '/api/jobs/a');

    expect(fetchMock).toHaveBeenCalledTimes(5);
    for (let i = 0; i < 5; i++) {
      expect(headerOf(requestOf(fetchMock, i).init, 'X-MM')).toBe('1');
    }
    expect(requestOf(fetchMock, 0).init.method).toBe('GET');
    expect(requestOf(fetchMock, 4).init.method).toBe('DELETE');
  });

  it('sets Content-Type only for a JSON body', async () => {
    fetchMock.mockImplementation(() => Promise.resolve(jsonResponse({})));
    const form = new FormData();
    form.append('files', new Blob(['x']), 'a.mp3');
    const picture = new Blob(['x'], { type: 'image/png' });

    await get('/api/info');
    await send('POST', '/api/undo');
    await send('POST', '/api/remove', { ids: ['a'] });
    await send('POST', '/api/ingest', form);
    await send('PUT', '/api/cover', picture);

    expect(headerOf(requestOf(fetchMock, 0).init, 'Content-Type')).toBeNull();
    expect(headerOf(requestOf(fetchMock, 1).init, 'Content-Type')).toBeNull();
    expect(requestOf(fetchMock, 1).init.body).toBeUndefined();
    expect(headerOf(requestOf(fetchMock, 2).init, 'Content-Type')).toBe('application/json');
    expect(requestOf(fetchMock, 2).init.body).toBe('{"ids":["a"]}');
    expect(headerOf(requestOf(fetchMock, 3).init, 'Content-Type')).toBeNull();
    expect(requestOf(fetchMock, 3).init.body).toBe(form);
    expect(headerOf(requestOf(fetchMock, 4).init, 'Content-Type')).toBeNull();
    expect(requestOf(fetchMock, 4).init.body).toBe(picture);
  });

  it('returns the parsed body', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ root: '/music' }));

    expect(await get<{ root: string }>('/api/info')).toEqual({ root: '/music' });
    expect(requestOf(fetchMock).url).toBe('/api/info');
  });

  it('returns null when the answer has no content', async () => {
    fetchMock.mockResolvedValue(new Response(null, { status: 204 }));

    expect(await send<null>('DELETE', '/api/jobs/a')).toBeNull();
  });

  it('turns an error body into the message', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ error: 'no such track' }, 422));

    const err = await get('/api/tracks/zz').catch((e: unknown) => e);

    expect(err).toBeInstanceOf(ApiError);
    expect(err).toHaveProperty('message', 'no such track');
    expect(err).toHaveProperty('status', 422);
  });

  it('names the status when the error body is not JSON', async () => {
    fetchMock.mockResolvedValue(new Response('<h1>bad gateway</h1>', { status: 502, statusText: 'Bad Gateway', headers: { 'Content-Type': 'text/html' } }));

    const err = await get('/api/info').catch((e: unknown) => e);

    expect(err).toBeInstanceOf(ApiError);
    expect(err).toHaveProperty('message', '502 Bad Gateway');
    expect(err).toHaveProperty('status', 502);
  });

  it('names the status when the error body has no message', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ detail: 'x' }, 500));

    const err = await get('/api/info').catch((e: unknown) => e);

    expect(err).toBeInstanceOf(ApiError);
    expect(err).toHaveProperty('message', 'Request failed (500)');
  });

  it('turns a failed fetch into the unreachable message', async () => {
    fetchMock.mockRejectedValue(new TypeError('Failed to fetch'));

    const err = await get('/api/info').catch((e: unknown) => e);

    expect(err).toBeInstanceOf(ApiError);
    expect(err).toHaveProperty('message', 'Cannot reach mm. Is it still running?');
    expect(err).toHaveProperty('status', 0);
  });

  it('rethrows AbortError', async () => {
    const abort = new DOMException('The operation was aborted.', 'AbortError');
    fetchMock.mockRejectedValue(abort);

    const err = await get('/api/library').catch((e: unknown) => e);

    expect(err).toBe(abort);
    expect(err).not.toBeInstanceOf(ApiError);
  });

  it('hands the abort signal to fetch', async () => {
    fetchMock.mockResolvedValue(jsonResponse({}));
    const controller = new AbortController();

    await get('/api/library', { signal: controller.signal });

    expect(requestOf(fetchMock).init.signal).toBe(controller.signal);
  });
});

describe('getBlob', () => {
  it('returns the bytes of the answer', async () => {
    fetchMock.mockResolvedValue(new Response(new Blob([new Uint8Array([1, 2, 3])]), { headers: { 'Content-Type': 'audio/mpeg' } }));

    const blob = await getBlob('/api/tracks/a/preview?field=cutStart');

    expect(requestOf(fetchMock).url).toBe('/api/tracks/a/preview?field=cutStart');
    expect(headerOf(requestOf(fetchMock).init, 'X-MM')).toBe('1');
    expect([...new Uint8Array(await blob.arrayBuffer())]).toEqual([1, 2, 3]);
  });

  it('turns an error body into the message', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ error: 'ffmpeg is not installed' }, 500));

    await expect(getBlob('/api/tracks/a/preview')).rejects.toEqual(new ApiError('ffmpeg is not installed', 500));
  });

  it('turns a failed fetch into the unreachable message', async () => {
    fetchMock.mockRejectedValue(new TypeError('Failed to fetch'));

    await expect(getBlob('/api/tracks/a/preview')).rejects.toThrow('Cannot reach mm. Is it still running?');
  });
});

describe('query', () => {
  it('skips empty, null and false and writes true as 1', () => {
    expect(query({ q: 'bach', empty: '', none: null, missing: undefined, off: false, on: true, zero: 0 })).toBe('?q=bach&on=1&zero=0');
  });

  it('is empty when nothing is left', () => {
    expect(query({})).toBe('');
    expect(query({ q: '', reverse: false, sort: null })).toBe('');
  });

  it('encodes keys and values', () => {
    expect(query({ q: 'a&b=c', 'k y': 'é' })).toBe('?q=a%26b%3Dc&k+y=%C3%A9');
  });
});
