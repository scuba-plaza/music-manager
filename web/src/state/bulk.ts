import { create } from 'zustand';
import { fieldErrors, send } from '../api/client';
import type { BulkPreview, ChangeResponse } from '../api/types.gen';
import { applyChange } from './edit';
import { cleanChecks, firstWrong, left, tried, verdict, type FormChecks } from './form';
import { closeOverlay, useApp } from './store';
import { messageOf } from './text';

export const bulkFields = [
  { key: 'artist', label: 'Artist' },
  { key: 'artistLatin', label: 'Artist (Latin)' },
  { key: 'album', label: 'Album' },
  { key: 'albumLatin', label: 'Album (Latin)' },
  { key: 'of', label: 'Tracks on album' },
  { key: 'released', label: 'Released' },
  { key: 'genre', label: 'Genre' },
  { key: 'genreLatin', label: 'Genre (Latin)' },
  { key: 'cutStart', label: 'Cut start' },
  { key: 'cutEnd', label: 'Cut end' },
] as const;

type BulkKey = (typeof bulkFields)[number]['key'];

type BulkValues = Record<BulkKey, string>;

interface BulkSummary {
  text: string;
  bad: boolean;
}

const previewPause = 150;
const prompt = 'Type what the tracks should have.';

function blank(): BulkValues {
  return { artist: '', artistLatin: '', album: '', albumLatin: '', of: '', released: '', genre: '', genreLatin: '', cutStart: '', cutEnd: '' };
}

interface BulkState {
  values: BulkValues;
  summary: BulkSummary;
  checks: FormChecks;
}

function fresh(): BulkState {
  return { values: blank(), summary: { text: prompt, bad: false }, checks: cleanChecks() };
}

export const useBulk = create<BulkState>()(fresh);

const order: readonly BulkKey[] = bulkFields.map((f) => f.key);

let timer: ReturnType<typeof setTimeout> | undefined;
let seq = 0;

function stop(): void {
  clearTimeout(timer);
  seq += 1;
}

function shownIds(): string[] | undefined {
  const overlay = useApp.getState().overlay;
  return overlay?.kind === 'bulk' ? overlay.ids : undefined;
}

function say(summary: BulkSummary): void {
  useBulk.setState({ summary });
}

export function openBulk(ids: string[], head: string): void {
  stop();
  useBulk.setState(fresh());
  useApp.setState({ overlay: { kind: 'bulk', ids, head } });
}

async function preview(): Promise<void> {
  const ids = shownIds();
  if (ids === undefined) {
    return;
  }
  seq += 1;
  const mine = seq;
  try {
    const answer = await send<BulkPreview>('POST', '/api/bulk/preview', { ids, values: useBulk.getState().values });
    if (mine === seq) {
      useBulk.setState((s) => ({ summary: { text: answer.summary || prompt, bad: false }, checks: verdict(s.checks, answer.fields ?? {}) }));
    }
  } catch (err) {
    if (mine === seq) {
      say({ text: messageOf(err), bad: true });
    }
  }
}

export function typeBulk(key: BulkKey, text: string): void {
  useBulk.setState((s) => ({ values: { ...s.values, [key]: text } }));
  clearTimeout(timer);
  timer = setTimeout(() => {
    void preview();
  }, previewPause);
}

export function leaveBulk(key: BulkKey): void {
  useBulk.setState((s) => ({ checks: left(s.checks, key) }));
}

export async function applyBulk(): Promise<BulkKey | undefined> {
  const ids = shownIds();
  if (ids === undefined) {
    return undefined;
  }
  const { values } = useBulk.getState();
  try {
    await applyChange(() => send<ChangeResponse>('POST', '/api/bulk', { ids, values }));
  } catch (err) {
    const wrong = fieldErrors(err);
    if (wrong !== undefined) {
      useBulk.setState((s) => ({ checks: tried(verdict(s.checks, wrong)) }));
      return firstWrong(useBulk.getState().checks, order);
    }
    if (err instanceof Error && err.message !== '') {
      say({ text: err.message, bad: true });
    }
    return undefined;
  }
  closeOverlay();
  return undefined;
}

useApp.subscribe((state, previous) => {
  if (previous.overlay?.kind === 'bulk' && state.overlay !== previous.overlay) {
    stop();
  }
});
