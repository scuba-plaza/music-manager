import { create } from 'zustand';
import type { InfoView, LibraryResponse, PeekView, ProfileView, TrackView } from '../api/types.gen';

export type Focus = 'tree' | 'inspector';

export type OverlayState =
  | { kind: 'ask'; head: string; body: string[]; yes: string; onYes: () => void }
  | { kind: 'help' }
  | { kind: 'sort' }
  | { kind: 'playlist'; ids: string[] }
  | { kind: 'copy'; id: string; title: string }
  | { kind: 'bulk'; ids: string[] }
  | { kind: 'identify' }
  | { kind: 'add' }
  | { kind: 'doctor' }
  | { kind: 'sync' };

export interface EditState {
  field: number;
  text: string;
  second?: string;
  note?: string;
  error?: string;
}

export interface AppState {
  info: InfoView | null;
  lib: LibraryResponse | null;
  filter: string;
  filterFocused: boolean;
  sort: { key: string; reverse: boolean };
  expanded: Set<string>;
  cursor: string | null;
  marks: Set<string>;
  focus: Focus;
  field: number;
  tab: number;
  detail: TrackView | PeekView | null;
  detailError: string;
  edit: EditState | null;
  playing: { id: string; field: string; src?: string } | null;
  profiles: ProfileView[];
  profile: string;
  status: { text: string; kind: '' | 'bad' | 'good' };
  overlay: OverlayState | null;
  busy: boolean;
}

const expandedKey = 'mm.expanded';
const tabKey = 'mm.tab';

export function recall(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

export function remember(key: string, value: string): void {
  try {
    localStorage.setItem(key, value);
  } catch {
    return;
  }
}

function recallExpanded(): Set<string> {
  try {
    const stored: unknown = JSON.parse(recall(expandedKey) ?? '[]');
    return new Set(Array.isArray(stored) ? stored.filter((key): key is string => typeof key === 'string') : []);
  } catch {
    return new Set();
  }
}

function recallTab(): number {
  const tab = Number(recall(tabKey) ?? 0);
  return Number.isInteger(tab) && tab >= 0 ? tab : 0;
}

export const useApp = create<AppState>()(() => ({
  info: null,
  lib: null,
  filter: '',
  filterFocused: false,
  sort: { key: '', reverse: false },
  expanded: recallExpanded(),
  cursor: null,
  marks: new Set(),
  focus: 'tree',
  field: 0,
  tab: recallTab(),
  detail: null,
  detailError: '',
  edit: null,
  playing: null,
  profiles: [],
  profile: '',
  status: { text: '', kind: '' },
  overlay: null,
  busy: false,
}));

useApp.subscribe((state, previous) => {
  if (state.expanded !== previous.expanded) {
    remember(expandedKey, JSON.stringify([...state.expanded]));
  }
  if (state.tab !== previous.tab) {
    remember(tabKey, String(state.tab));
  }
});
