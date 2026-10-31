import { create } from 'zustand';
import { send } from '../api/client';
import type { BulkPreview, ChangeResponse } from '../api/types.gen';
import { applyChange } from './edit';
import { useApp } from './store';

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

export type BulkKey = (typeof bulkFields)[number]['key'];

export type BulkValues = Record<BulkKey, string>;

export interface BulkSummary {
  text: string;
  bad: boolean;
}

const previewPause = 150;
const prompt = 'Type what the tracks should have.';

function blank(): BulkValues {
  return { artist: '', artistLatin: '', album: '', albumLatin: '', of: '', released: '', genre: '', genreLatin: '', cutStart: '', cutEnd: '' };
}

function fresh(): { values: BulkValues; summary: BulkSummary } {
  return { values: blank(), summary: { text: prompt, bad: false } };
}

export const useBulk = create<{ values: BulkValues; summary: BulkSummary }>()(fresh);

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
      say(answer.error !== undefined && answer.error !== '' ? { text: answer.error, bad: true } : { text: answer.summary || prompt, bad: false });
    }
  } catch (err) {
    if (mine === seq) {
      say({ text: err instanceof Error ? err.message : String(err), bad: true });
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

export async function applyBulk(): Promise<void> {
  const ids = shownIds();
  if (ids === undefined) {
    return;
  }
  const { values } = useBulk.getState();
  try {
    await applyChange(() => send<ChangeResponse>('POST', '/api/bulk', { ids, values }));
  } catch (err) {
    if (err instanceof Error && err.message !== '') {
      say({ text: err.message, bad: true });
    }
    return;
  }
  useApp.setState({ overlay: null });
}

useApp.subscribe((state, previous) => {
  if (previous.overlay?.kind === 'bulk' && state.overlay !== previous.overlay) {
    stop();
  }
});
