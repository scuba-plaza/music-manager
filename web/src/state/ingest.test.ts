import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { changeResponse, infoView, jsonResponse, memoryStorage, sampleLibrary, stubServer, type StubRoute , urlOf } from '../testing/fakes';
import { dropFiles } from './cover';
import { uploadFiles } from './ingest';
import { useApp } from './store';

let fetchMock: ReturnType<typeof stubServer>;

const ingest: StubRoute = ({ method, url }) =>
  method === 'POST' && url.pathname === '/api/ingest' ? jsonResponse(changeResponse({ message: 'Added 2 tracks · u undoes it', reveal: 'new1' })) : undefined;

function serve(route: StubRoute = ingest): void {
  fetchMock = stubServer(infoView(), sampleLibrary(), { route });
}

function mp3(name: string): File {
  return new File(['ID3'], name, { type: 'audio/mpeg' });
}

function uploads(): { url: string; body: FormData }[] {
  return fetchMock.mock.calls.flatMap(([input, init]) => (urlOf(input) === '/api/ingest' && init?.body instanceof FormData ? [{ url: urlOf(input), body: init.body }] : []));
}

beforeEach(() => {
  vi.stubGlobal('localStorage', memoryStorage());
  useApp.setState(useApp.getInitialState(), true);
  serve();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('adding files', () => {
  it('sends the MP3 files, each under its own name, and says what the server made of them', async () => {
    await uploadFiles([mp3('one.mp3'), mp3('Two.MP3')]);

    const [sent] = uploads();
    expect(uploads()).toHaveLength(1);
    expect(sent?.body.getAll('files').map((f) => (f instanceof File ? f.name : f))).toEqual(['one.mp3', 'Two.MP3']);
    expect(useApp.getState().status).toEqual({ text: 'Added 2 tracks · u undoes it', kind: '' });
    expect(useApp.getState().busy).toBe(false);
  });

  it('says what is being added while the server works', async () => {
    let answer: (res: Response) => void = () => undefined;
    serve(({ method, url }) =>
      method === 'POST' && url.pathname === '/api/ingest'
        ? new Promise<Response>((resolve) => {
            answer = resolve;
          })
        : undefined,
    );

    const adding = uploadFiles([mp3('one.mp3'), mp3('two.mp3')]);

    expect(useApp.getState().status).toEqual({ text: 'Adding 2 files…', kind: '' });
    answer(jsonResponse(changeResponse({ message: 'Added 2 tracks' })));
    await adding;
    expect(useApp.getState().status.text).toBe('Added 2 tracks');
  });

  it('leaves out what is not an MP3 file', async () => {
    await uploadFiles([new File(['x'], 'notes.txt', { type: 'text/plain' }), mp3('one.mp3')]);

    expect(uploads()[0]?.body.getAll('files').map((f) => (f instanceof File ? f.name : f))).toEqual(['one.mp3']);
  });

  it('says so, and sends nothing, when none of the files is an MP3 file', async () => {
    await uploadFiles([new File(['x'], 'notes.txt', { type: 'text/plain' })]);

    expect(uploads()).toEqual([]);
    expect(useApp.getState().status).toEqual({ text: 'Only MP3 files can be added', kind: 'bad' });
  });

  it('says what went wrong when the server refuses', async () => {
    serve(({ method, url }) => (method === 'POST' && url.pathname === '/api/ingest' ? jsonResponse({ error: 'the files could not be read, or are too large' }, 413) : undefined));

    await uploadFiles([mp3('one.mp3')]);

    expect(useApp.getState().status).toEqual({ text: 'the files could not be read, or are too large', kind: 'bad' });
    expect(useApp.getState().busy).toBe(false);
  });
});

describe('dropping files on the page', () => {
  it('adds the MP3 files, even with a picture among them', async () => {
    await dropFiles([new File(['x'], 'front.png', { type: 'image/png' }), mp3('one.mp3')]);

    expect(uploads()).toHaveLength(1);
    expect(uploads()[0]?.body.getAll('files').map((f) => (f instanceof File ? f.name : f))).toEqual(['one.mp3']);
    expect(fetchMock.mock.calls.filter(([, init]) => init?.method === 'PUT')).toEqual([]);
  });
});
