import { create } from 'zustand';
import { send } from '../api/client';
import type { CandidateView, ChangeResponse, SearchView } from '../api/types.gen';
import { loadLibrary } from './library';
import { setStatus } from './status';
import { applyChange } from './edit';
import { useApp } from './store';
import { rowByKey } from './tree';

export type IdentifyPhase = 'searching' | 'listing' | 'failed' | 'editing' | 'applying';

export interface SearchWords {
  title: string;
  artist: string;
}

export interface IdentifyState {
  phase: IdentifyPhase;
  id: string;
  words: SearchWords;
  candidates: CandidateView[];
  found: string;
  cursor: number;
  error: string;
}

const coverPause = 600;

function fresh(): IdentifyState {
  return { phase: 'searching', id: '', words: { title: '', artist: '' }, candidates: [], found: '', cursor: 0, error: '' };
}

export const useIdentify = create<IdentifyState>()(fresh);

let current: AbortController | null = null;

function cancel(): void {
  current?.abort();
  current = null;
}

function messageOf(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

export function needLookup(): boolean {
  if (useApp.getState().info?.lookup === true) {
    return true;
  }
  setStatus('Lookup is not available: no MusicBrainz client is set up', 'bad');
  return false;
}

async function search(words?: SearchWords): Promise<void> {
  cancel();
  const mine = new AbortController();
  current = mine;
  const { id } = useIdentify.getState();
  useIdentify.setState({ phase: 'searching', error: '', ...(words === undefined ? {} : { words }) });
  try {
    const found = await send<SearchView>('POST', `/api/tracks/${encodeURIComponent(id)}/candidates`, words === undefined ? {} : { query: words }, {
      signal: mine.signal,
    });
    if (current !== mine) {
      return;
    }
    const heard = { title: found.title, artist: found.artist };
    if (found.candidates.length === 0) {
      useIdentify.setState({ phase: 'failed', words: heard, candidates: [], error: `Nothing found on MusicBrainz for “${found.title} – ${found.artist}”` });
      return;
    }
    useIdentify.setState({ phase: 'listing', words: heard, candidates: found.candidates, found: found.search, cursor: 0 });
  } catch (err) {
    if (current !== mine) {
      return;
    }
    useIdentify.setState({ phase: 'failed', error: messageOf(err) });
  }
}

export function identify(): void {
  const { lib, cursor } = useApp.getState();
  const row = rowByKey(lib, cursor);
  if (row?.trackId === undefined || !needLookup()) {
    return;
  }
  cancel();
  useIdentify.setState({ ...fresh(), id: row.trackId, words: { title: row.label, artist: '' } });
  useApp.setState({ overlay: { kind: 'identify' } });
  void search();
}

export function searchFor(title: string, artist: string): void {
  void search({ title: title.trim(), artist: artist.trim() });
}

export function editSearch(): void {
  useIdentify.setState({ phase: 'editing' });
}

export function cancelSearchEditing(): void {
  useIdentify.setState((s) => ({ phase: s.candidates.length > 0 ? 'listing' : 'failed' }));
}

export function chooseCandidate(i: number): void {
  useIdentify.setState((s) => ({ cursor: Math.min(Math.max(i, 0), s.candidates.length - 1) }));
}

export function moveCandidate(by: number): void {
  chooseCandidate(useIdentify.getState().cursor + by);
}

export async function applyCandidate(): Promise<void> {
  const { found, cursor } = useIdentify.getState();
  cancel();
  const mine = new AbortController();
  current = mine;
  useIdentify.setState({ phase: 'applying', error: '' });
  try {
    await applyChange(() => send<ChangeResponse>('POST', '/api/identify/apply', { search: found, index: cursor }, { signal: mine.signal }));
    if (current === mine) {
      current = null;
      useApp.setState({ overlay: null });
    }
  } catch (err) {
    if (current !== mine) {
      return;
    }
    const text = messageOf(err);
    useIdentify.setState(text === '' ? { phase: 'listing' } : { phase: 'failed', error: text });
  }
}

export function skipCover(): void {
  const { id } = useIdentify.getState();
  cancel();
  useApp.setState({ overlay: null });
  setStatus('Identified without waiting for the cover');
  setTimeout(() => {
    void loadLibrary({ reveal: id });
  }, coverPause);
}

useApp.subscribe((state, previous) => {
  if (previous.overlay?.kind === 'identify' && state.overlay?.kind !== 'identify') {
    cancel();
  }
});
