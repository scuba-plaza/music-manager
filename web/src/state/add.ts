import { create } from 'zustand';
import { send } from '../api/client';
import type { AddForm, ChangeResponse, DraftView, DuplicateView, Fetched, JobView, PathView } from '../api/types.gen';
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
  job: JobView | null;
  started: number;
  draft: DraftView | null;
  useMatch: boolean;
  form: AddForm;
  touched: ReadonlySet<AddKey>;
  existing: DuplicateView | null;
  pathNote: string;
}

const pathPause = 120;
const defaulted: readonly AddKey[] = ['title', 'artist', 'album', 'track', 'of', 'released'];

function blank(): AddForm {
  return { title: '', artist: '', album: '', track: '', of: '', released: '', genre: '', cutStart: '', cutEnd: '' };
}

function fresh(): AddState {
  return {
    phase: 'asking',
    link: '',
    error: '',
    job: null,
    started: 0,
    draft: null,
    useMatch: true,
    form: blank(),
    touched: new Set(),
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

function filled(form: AddForm, values: AddForm, touched: ReadonlySet<AddKey>): AddForm {
  const next = { ...form };
  for (const key of defaulted) {
    if (!touched.has(key)) {
      next[key] = values[key];
    }
  }
  return next;
}

function addOpen(): boolean {
  return useApp.getState().overlay?.kind === 'add';
}

function askForPath(): void {
  clearTimeout(pathTimer);
  pathTimer = setTimeout(() => {
    pathNo += 1;
    const mine = pathNo;
    send<PathView>('POST', '/api/add/path', { form: useAdd.getState().form }).then(
      (res) => {
        if (mine === pathNo) {
          useAdd.setState({ pathNote: res.path });
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
  useAdd.setState({ link: text, error: '' });
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
  const useMatch = draft.match !== undefined;
  useAdd.setState({
    phase: 'form',
    draft,
    useMatch,
    touched: new Set(),
    form: { ...filled(blank(), useMatch ? draft.matched : draft.downloaded, new Set()), genre: '', cutStart: '', cutEnd: '' },
  });
  askForPath();
}

export function fetchLink(): void {
  const link = useAdd.getState().link.trim();
  if (link === '') {
    useAdd.setState({ error: 'Paste a link first' });
    return;
  }
  attemptNo += 1;
  const mine = attemptNo;
  useAdd.setState({ phase: 'fetching', link, error: '', job: null, started: Date.now() });
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
      if (mine === attemptNo) {
        useAdd.setState({ phase: 'failed', error: messageOf(err) });
      }
    },
  );
}

export function retryAdd(): void {
  fetchLink();
}

export function setField(key: AddKey, text: string): void {
  useAdd.setState((s) => ({ form: { ...s.form, [key]: text }, touched: new Set(s.touched).add(key) }));
  askForPath();
}

export function toggleMatch(): void {
  const { draft, useMatch, form, touched } = useAdd.getState();
  if (draft?.match === undefined) {
    return;
  }
  const next = !useMatch;
  useAdd.setState({ useMatch: next, form: filled(form, next ? draft.matched : draft.downloaded, touched) });
  askForPath();
}

export async function saveAdd(): Promise<void> {
  const { draft, form } = useAdd.getState();
  if (draft === null) {
    return;
  }
  useAdd.setState({ phase: 'saving', error: '' });
  try {
    await applyChange(() => send<ChangeResponse>('POST', '/api/add/finish', { draft: draft.draft, form }));
  } catch (err) {
    useAdd.setState({ phase: 'form', error: messageOf(err) });
    return;
  }
  finished = true;
  useApp.setState({ overlay: null });
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
