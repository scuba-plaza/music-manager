import { create } from 'zustand';
import { ApiError, send } from '../api/client';
import type { AddCheck, AddForm, ChangeResponse, DraftCoverView, DraftView, DuplicateView, Fetched, JobView } from '../api/types.gen';
import { cleanChecks, firstWrong, left, tried, verdict, type FormChecks } from './form';
import { loadLibrary } from './library';
import { setStatus } from './status';
import { applyChange } from './edit';
import { uploadFiles } from './ingest';
import { follow, stopJob } from './jobs';
import { useApp } from './store';

export type AddPhase = 'asking' | 'fetching' | 'form' | 'saving' | 'duplicate' | 'failed';

export type AddKey = keyof AddForm;

export interface AddState {
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
}

const pathPause = 120;

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
  };
}

export const useAdd = create<AddState>()(fresh);

let attemptNo = 0;
let pathTimer: ReturnType<typeof setTimeout> | undefined;
let pathNo = 0;
let finished = false;

function messageOf(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

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

export function retryAdd(): void {
  fetchLink();
}

export function setField(key: AddKey, text: string): void {
  useAdd.setState((s) => ({ form: { ...s.form, [key]: text } }));
  askForPath();
}

const enc = encodeURIComponent;

export async function chooseDraftCover(file: File): Promise<void> {
  const { draft } = useAdd.getState();
  if (draft === null) {
    return;
  }
  try {
    const res = await send<DraftCoverView>('PUT', `/api/add/${enc(draft.draft)}/cover`, file);
    useAdd.setState({ cover: res.cover, coverError: '' });
  } catch (err) {
    useAdd.setState({ coverError: messageOf(err) });
  }
}

export async function removeDraftCover(): Promise<void> {
  const { draft } = useAdd.getState();
  if (draft === null) {
    return;
  }
  try {
    const res = await send<DraftCoverView>('DELETE', `/api/add/${enc(draft.draft)}/cover`);
    useAdd.setState({ cover: res.cover, coverError: '' });
  } catch (err) {
    useAdd.setState({ coverError: messageOf(err) });
  }
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
    if (err instanceof ApiError && Object.keys(err.fields).length > 0) {
      useAdd.setState((s) => ({ phase: 'form', checks: tried(verdict(s.checks, err.fields)) }));
      const first = firstWrong(useAdd.getState().checks, addOrder);
      return addOrder.find((key) => key === first);
    }
    useAdd.setState({ phase: 'form', error: messageOf(err) });
    return undefined;
  }
  finished = true;
  useApp.setState({ overlay: null });
  return undefined;
}

export function showDuplicate(): void {
  const { existing } = useAdd.getState();
  if (existing === null) {
    return;
  }
  useApp.setState({ overlay: null });
  void loadLibrary({ reveal: existing.id });
}

export function leaveAdd(): void {
  useApp.setState({ overlay: null });
}

export function chooseFiles(files: readonly File[]): void {
  if (files.length === 0) {
    return;
  }
  useApp.setState({ overlay: null });
  void uploadFiles(files);
}

useApp.subscribe((state, previous) => {
  if (previous.overlay?.kind !== 'add' || state.overlay?.kind === 'add') {
    return;
  }
  clearTimeout(pathTimer);
  const { phase, job, draft } = useAdd.getState();
  if (phase === 'fetching' && job !== null) {
    void stopJob(job.id);
  }
  if (draft !== null && !finished) {
    send('DELETE', `/api/add/${encodeURIComponent(draft.draft)}`).catch(() => undefined);
  }
  finished = true;
});
