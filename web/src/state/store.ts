import { create } from 'zustand';
import type { CopyChoice, InfoView, LibraryResponse, PeekView, ProfileView, TrackView } from '../api/types.gen';
import type { Hint } from '../keys/keymap';

export type Focus = 'tree' | 'inspector';

export type OverlayState =
  | { kind: 'ask'; head: string; body: string[]; yes: string; onYes: () => void }
  | { kind: 'help' }
  | { kind: 'sort' }
  | { kind: 'playlist'; ids: string[] }
  | { kind: 'copy'; id: string; title: string; choices: CopyChoice[] }
  | { kind: 'bulk'; ids: string[]; head: string }
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

export function closeOverlay(): void {
  useApp.setState({ overlay: null });
}

export function ask(head: string, body: string[], yes: string, onYes: () => void): void {
  useApp.setState({ overlay: { kind: 'ask', head, body, yes, onYes } });
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
  dialogHints: readonly Hint[];
  helpPage: number;
  busy: boolean;
}

const expandedKey = 'mm.expanded';

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
  tab: 0,
  detail: null,
  detailError: '',
  edit: null,
  playing: null,
  profiles: [],
  profile: '',
  status: { text: '', kind: '' },
  overlay: null,
  dialogHints: [],
  helpPage: 0,
  busy: false,
}));

useApp.subscribe((state, previous) => {
  if (state.expanded !== previous.expanded) {
    remember(expandedKey, JSON.stringify([...state.expanded]));
  }
});
