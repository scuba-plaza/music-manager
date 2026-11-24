import { create } from 'zustand';
import { ApiError, fieldErrors, send } from '../api/client';
import type { AddCheck, AddForm, AlbumForm, AlbumProgress, AlbumView, ChangeResponse, DraftCoverView, DraftView, DuplicateView, Fetched, JobView } from '../api/types.gen';
import { cleanChecks, firstWrong, left, tried, verdict, type FormChecks } from './form';
import { loadLibrary } from './library';
import { setStatus } from './status';
import { applyChange } from './edit';
import { uploadFiles } from './ingest';
import { follow, stopJob } from './jobs';
import { closeOverlay, useApp } from './store';
import { messageOf } from './text';

export type AddPhase = 'asking' | 'fetching' | 'form' | 'saving' | 'duplicate' | 'failed' | 'album' | 'adding';

export type AddKey = keyof AddForm;

export type AlbumKey = keyof AlbumForm;

interface AddState {
  phase: AddPhase;
  link: string;
  error: string;
  linkError: string;
  cover: string;
  coverError: string;
  checks: FormChecks;
  job: JobView | null;
  started: number;
  draft: DraftView | null;
  form: AddForm;
  existing: DuplicateView | null;
  pathNote: string;
  album: AlbumView | null;
  albumForm: AlbumForm;
  albumErrors: Record<string, string>;
  picked: readonly number[];
  progress: AlbumProgress | null;
}

const pathPause = 120;

const pickOne = 'Pick at least one track';

export const albumOrder: readonly AlbumKey[] = ['album', 'artist', 'released', 'genre'];

function blankAlbum(): AlbumForm {
  return { album: '', artist: '', released: '', genre: '' };
}

export const addOrder: readonly AddKey[] = ['title', 'artist', 'album', 'track', 'of', 'released', 'genre', 'cutStart', 'cutEnd'];

function blank(): AddForm {
  return { title: '', artist: '', album: '', track: '', of: '', released: '', genre: '', cutStart: '', cutEnd: '' };
}

function fresh(): AddState {
  return {
    phase: 'asking',
    link: '',
    error: '',
    linkError: '',
    cover: '',
    coverError: '',
    checks: cleanChecks(),
    job: null,
    started: 0,
    draft: null,
    form: blank(),
    existing: null,
    pathNote: '',
    album: null,
    albumForm: blankAlbum(),
    albumErrors: {},
    picked: [],
    progress: null,
  };
}

export const useAdd = create<AddState>()(fresh);

let attemptNo = 0;
let pathTimer: ReturnType<typeof setTimeout> | undefined;
let pathNo = 0;
let finished = false;

function addOpen(): boolean {
  return useApp.getState().overlay?.kind === 'add';
}

function askForPath(): void {
  clearTimeout(pathTimer);
  pathTimer = setTimeout(() => {
    pathNo += 1;
    const mine = pathNo;
    send<AddCheck>('POST', '/api/add/path', { form: useAdd.getState().form }).then(
      (res) => {
        if (mine === pathNo) {
          useAdd.setState((s) => ({ pathNote: res.path, checks: verdict(s.checks, res.fields ?? {}) }));
        }
      },
      () => {
        if (mine === pathNo) {
          useAdd.setState({ pathNote: '' });
        }
      },
    );
  }, pathPause);
}

export function openAdd(): void {
  if (useApp.getState().info?.canAdd !== true) {
    setStatus('Adding from YouTube is not available: yt-dlp is not set up', 'bad');
    return;
  }
  attemptNo += 1;
  pathNo += 1;
  clearTimeout(pathTimer);
  finished = false;
  useAdd.setState(fresh());
  useApp.setState({ overlay: { kind: 'add' } });
}

export function typeLink(text: string): void {
  useAdd.setState({ link: text, linkError: '' });
}

function fetchedOf(data: unknown): Fetched {
  return typeof data === 'object' && data !== null ? data : {};
}

function receive(ended: JobView): void {
  if (ended.state === 'failed') {
    useAdd.setState({ phase: 'failed', error: ended.error ?? '' });
    return;
  }
  const out = fetchedOf(ended.data);
  if (out.duplicate !== undefined) {
    useAdd.setState({ phase: 'duplicate', existing: out.duplicate });
    return;
  }
  if (out.album !== undefined) {
    useAdd.setState({
      phase: 'album',
      album: out.album,
      picked: out.album.tracks.map((t) => t.number),
      albumForm: out.album.form,
      albumErrors: {},
      cover: out.album.cover,
      coverError: '',
    });
    return;
  }
  const draft = out.draft;
  if (draft === undefined) {
    return;
  }
  useAdd.setState({ phase: 'form', draft, form: draft.form, cover: draft.cover, coverError: '' });
  askForPath();
}

export function fetchLink(): void {
  const link = useAdd.getState().link.trim();
  if (link === '') {
    useAdd.setState({ linkError: 'Paste a link first' });
    return;
  }
  attemptNo += 1;
  const mine = attemptNo;
  useAdd.setState({ phase: 'fetching', link, error: '', linkError: '', job: null, started: Date.now() });
  send<JobView>('POST', '/api/add/fetch', { url: link }).then(
    async (view) => {
      if (mine !== attemptNo) {
        return;
      }
      useAdd.setState({ job: view });
      const ended = await follow(view, (v) => {
        if (mine === attemptNo) {
          useAdd.setState({ job: v });
        }
      });
      if (mine === attemptNo && addOpen()) {
        receive(ended);
      }
    },
    (err: unknown) => {
      if (mine !== attemptNo) {
        return;
      }
      const linkError = err instanceof ApiError ? err.fields.link : undefined;
      useAdd.setState(linkError === undefined ? { phase: 'failed', error: messageOf(err) } : { phase: 'asking', linkError });
    },
  );
}

export function setField(key: AddKey, text: string): void {
  useAdd.setState((s) => ({ form: { ...s.form, [key]: text } }));
  askForPath();
}

async function changeDraftCover(method: 'PUT' | 'DELETE', file?: File): Promise<void> {
  const { draft } = useAdd.getState();
  if (draft === null) {
    return;
  }
  try {
    const res = await send<DraftCoverView>(method, `/api/add/${encodeURIComponent(draft.draft)}/cover`, file);
    useAdd.setState({ cover: res.cover, coverError: '' });
  } catch (err) {
    useAdd.setState({ coverError: messageOf(err) });
  }
}

export function chooseDraftCover(file: File): Promise<void> {
  return changeDraftCover('PUT', file);
}

export function removeDraftCover(): Promise<void> {
  return changeDraftCover('DELETE');
}

export function leaveField(key: AddKey): void {
  useAdd.setState((s) => ({ checks: left(s.checks, key) }));
}

export async function saveAdd(): Promise<AddKey | undefined> {
  const { draft, form } = useAdd.getState();
  if (draft === null) {
    return undefined;
  }
  useAdd.setState({ phase: 'saving', error: '' });
  try {
    await applyChange(() => send<ChangeResponse>('POST', '/api/add/finish', { draft: draft.draft, form }));
  } catch (err) {
    const wrong = fieldErrors(err);
    if (wrong !== undefined) {
      useAdd.setState((s) => ({ phase: 'form', checks: tried(verdict(s.checks, wrong)) }));
      return firstWrong(useAdd.getState().checks, addOrder);
    }
    useAdd.setState({ phase: 'form', error: messageOf(err) });
    return undefined;
  }
  finished = true;
  closeOverlay();
  return undefined;
}

function progressOf(data: unknown): AlbumProgress | null {
  if (typeof data !== 'object' || data === null) {
    return null;
  }
  if ('done' in data && 'total' in data && 'title' in data) {
    const { done, total, title } = data;
    return typeof done === 'number' && typeof total === 'number' && typeof title === 'string' ? { done, total, title } : null;
  }
  return null;
}

function changeOf(data: unknown): ChangeResponse {
  if (typeof data === 'object' && data !== null && 'history' in data && typeof data.history === 'object' && data.history !== null) {
    return { ...data, history: data.history };
  }
  return { history: {} };
}

export function setAlbumField(key: AlbumKey, text: string): void {
  useAdd.setState((s) => ({
    albumForm: { ...s.albumForm, [key]: text },
    albumErrors: Object.fromEntries(Object.entries(s.albumErrors).filter(([name]) => name !== key)),
  }));
}

export function togglePick(number: number): void {
  useAdd.setState((s) => {
    const picked = s.picked.includes(number) ? s.picked.filter((n) => n !== number) : [...s.picked, number].sort((a, b) => a - b);
    return { picked, error: picked.length > 0 && s.error === pickOne ? '' : s.error };
  });
}

export function pickAll(all: boolean): void {
  useAdd.setState((s) => {
    const picked = all && s.album !== null ? s.album.tracks.map((t) => t.number) : [];
    return { picked, error: picked.length > 0 && s.error === pickOne ? '' : s.error };
  });
}

export async function saveAlbum(): Promise<AlbumKey | undefined> {
  const { album, albumForm, picked } = useAdd.getState();
  if (album === null) {
    return undefined;
  }
  if (picked.length === 0) {
    useAdd.setState({ error: pickOne });
    return undefined;
  }
  attemptNo += 1;
  const mine = attemptNo;
  const tracks = [...picked].sort((a, b) => a - b);
  useAdd.setState({ phase: 'adding', error: '', albumErrors: {}, progress: null, job: null, started: Date.now() });
  let ended: JobView;
  try {
    const view = await send<JobView>('POST', '/api/add/album', { id: album.id, form: albumForm, tracks });
    if (mine !== attemptNo) {
      void stopJob(view.id);
      return undefined;
    }
    useAdd.setState({ job: view, progress: progressOf(view.data) });
    ended = await follow(view, (v) => {
      if (mine === attemptNo) {
        useAdd.setState({ job: v, progress: progressOf(v.data) });
      }
    });
  } catch (err) {
    if (mine !== attemptNo) {
      return undefined;
    }
    const wrong = fieldErrors(err);
    if (wrong !== undefined) {
      useAdd.setState({ phase: 'album', albumErrors: { ...wrong } });
      return albumOrder.find((key) => wrong[key] !== undefined);
    }
    useAdd.setState({ phase: 'album', error: messageOf(err) });
    return undefined;
  }
  if (mine !== attemptNo || !addOpen()) {
    return undefined;
  }
  if (ended.state !== 'done') {
    useAdd.setState({ phase: 'failed', error: ended.error ?? '' });
    return undefined;
  }
  finished = true;
  useAdd.setState({ job: null });
  try {
    await applyChange(() => Promise.resolve(changeOf(ended.data)));
  } catch {
    useAdd.setState({ error: '' });
  }
  closeOverlay();
  return undefined;
}

export function showDuplicate(): void {
  const { existing } = useAdd.getState();
  if (existing === null) {
    return;
  }
  closeOverlay();
  void loadLibrary({ reveal: existing.id });
}

export function chooseFiles(files: readonly File[]): void {
  if (files.length === 0) {
    return;
  }
  closeOverlay();
  void uploadFiles(files);
}

useApp.subscribe((state, previous) => {
  if (previous.overlay?.kind !== 'add' || state.overlay?.kind === 'add') {
    return;
  }
  clearTimeout(pathTimer);
  const { phase, job, draft, album } = useAdd.getState();
  if ((phase === 'fetching' || phase === 'adding') && job !== null) {
    void stopJob(job.id);
  }
  if (phase === 'adding' && job === null) {
    attemptNo += 1;
  }
  if (draft !== null && !finished) {
    send('DELETE', `/api/add/${encodeURIComponent(draft.draft)}`).catch(() => undefined);
  }
  if (album !== null && (phase === 'album' || phase === 'adding' || phase === 'failed') && !finished) {
    send('DELETE', `/api/add/${encodeURIComponent(album.id)}`).catch(() => undefined);
  }
  finished = true;
});
