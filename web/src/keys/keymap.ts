import { currentField, isTrack, nudgeable } from '../state/inspector';
import type { AppState } from '../state/store';

export type Scope = 'common' | 'tree' | 'inspector';

export type KeyEventLike = Pick<KeyboardEvent, 'key' | 'ctrlKey' | 'metaKey' | 'altKey' | 'shiftKey' | 'isComposing'>;

export interface Hint {
  keys: string;
  label: string;
}

export type HelpPage = 'Moving' | 'Editing' | 'Marking' | 'Library';

export type ActionName =
  | 'moveUp'
  | 'moveDown'
  | 'moveToTop'
  | 'moveToBottom'
  | 'openLevel'
  | 'closeAll'
  | 'open'
  | 'enterRow'
  | 'close'
  | 'markToggle'
  | 'markAll'
  | 'bulkEdit'
  | 'remove'
  | 'moveTrackDown'
  | 'moveTrackUp'
  | 'renumber'
  | 'addToPlaylist'
  | 'copyFrom'
  | 'fieldUp'
  | 'fieldDown'
  | 'tab1'
  | 'tab2'
  | 'tab3'
  | 'tab4'
  | 'startEdit'
  | 'clear'
  | 'nudgeDown'
  | 'nudgeUp'
  | 'nudgeDownMore'
  | 'nudgeUpMore'
  | 'hear'
  | 'copyField'
  | 'toggleFocus'
  | 'focusFilter'
  | 'sortMenu'
  | 'add'
  | 'sync'
  | 'formats'
  | 'undo'
  | 'redo'
  | 'doctor'
  | 'paste'
  | 'escape'
  | 'help'
  | 'filterAccept'
  | 'filterClear';

export interface Binding {
  keys: readonly string[];
  scope: Scope;
  action: ActionName;
  hint?: { label: string | ((s: AppState) => string); when?: (s: AppState) => boolean };
  help: { page: HelpPage; text: string };
}

function onTrack(s: AppState): boolean {
  return isTrack(s.detail);
}

function onCover(s: AppState): boolean {
  return currentField(s)?.key === 'cover';
}

function onCut(s: AppState): boolean {
  return nudgeable.includes(currentField(s)?.key ?? '');
}

function canEdit(s: AppState): boolean {
  return currentField(s)?.editable === true;
}

const tabs = { label: 'tabs', when: onTrack };
const tabHelp = { page: 'Editing', text: "a track's tab by its number: Details, Audio, Latin, Device" } as const;
const inTree = (s: AppState) => s.focus === 'tree';
const inInspector = (s: AppState) => s.focus === 'inspector';

export const bindings: readonly Binding[] = [
  { keys: ['ArrowUp', 'k'], scope: 'tree', action: 'moveUp', hint: { label: 'move' }, help: { page: 'Moving', text: 'move' } },
  { keys: ['ArrowDown', 'j'], scope: 'tree', action: 'moveDown', hint: { label: 'move' }, help: { page: 'Moving', text: 'move' } },
  { keys: ['Home', 'g'], scope: 'tree', action: 'moveToTop', help: { page: 'Moving', text: 'top / bottom' } },
  { keys: ['End', 'Shift+g'], scope: 'tree', action: 'moveToBottom', help: { page: 'Moving', text: 'top / bottom' } },
  { keys: ['Shift+ArrowRight'], scope: 'tree', action: 'openLevel', help: { page: 'Moving', text: 'open the whole tree one more level' } },
  { keys: ['Shift+ArrowLeft'], scope: 'tree', action: 'closeAll', help: { page: 'Moving', text: 'close the whole tree' } },
  {
    keys: ['ArrowRight', 'l'],
    scope: 'tree',
    action: 'open',
    hint: { label: 'fields' },
    help: { page: 'Moving', text: 'into the fields of a track, artist, album or playlist' },
  },
  { keys: ['Enter'], scope: 'tree', action: 'enterRow', help: { page: 'Moving', text: 'open / close; on a track, into its fields' } },
  { keys: ['ArrowLeft', 'h'], scope: 'tree', action: 'close', help: { page: 'Moving', text: 'close, or up a level' } },
  { keys: ['Space'], scope: 'tree', action: 'markToggle', hint: { label: 'mark' }, help: { page: 'Marking', text: 'mark a track (an album marks all its tracks)' } },
  { keys: ['Shift+Space'], scope: 'tree', action: 'markAll', help: { page: 'Marking', text: 'mark every track shown' } },
  {
    keys: ['e'],
    scope: 'tree',
    action: 'bulkEdit',
    hint: { label: 'edit' },
    help: { page: 'Marking', text: 'edit the marked tracks together; with none marked, into the fields' },
  },
  {
    keys: ['d', 'Delete'],
    scope: 'tree',
    action: 'remove',
    hint: { label: 'remove' },
    help: { page: 'Marking', text: 'remove (asks first); in a playlist, take a track out of it' },
  },
  { keys: ['Shift+ArrowDown'], scope: 'tree', action: 'moveTrackDown', help: { page: 'Marking', text: 'move a track down / up' } },
  { keys: ['Shift+ArrowUp'], scope: 'tree', action: 'moveTrackUp', help: { page: 'Marking', text: 'move a track down / up' } },
  { keys: ['n'], scope: 'tree', action: 'renumber', help: { page: 'Marking', text: 'number an album 1…N' } },
  { keys: ['p'], scope: 'tree', action: 'addToPlaylist', hint: { label: 'playlist' }, help: { page: 'Marking', text: 'add to a playlist' } },
  { keys: ['c'], scope: 'tree', action: 'copyFrom', help: { page: 'Marking', text: 'copy the cover, artist, album… of a track' } },

  { keys: ['ArrowUp', 'k'], scope: 'inspector', action: 'fieldUp', hint: { label: 'field' }, help: { page: 'Editing', text: 'choose a field' } },
  { keys: ['ArrowDown', 'j'], scope: 'inspector', action: 'fieldDown', hint: { label: 'field' }, help: { page: 'Editing', text: 'choose a field' } },
  { keys: ['1'], scope: 'inspector', action: 'tab1', hint: tabs, help: tabHelp },
  { keys: ['2'], scope: 'inspector', action: 'tab2', hint: tabs, help: tabHelp },
  { keys: ['3'], scope: 'inspector', action: 'tab3', hint: tabs, help: tabHelp },
  { keys: ['4'], scope: 'inspector', action: 'tab4', hint: tabs, help: tabHelp },
  {
    keys: ['Enter'],
    scope: 'inspector',
    action: 'startEdit',
    hint: { label: (s) => (onCover(s) ? 'choose' : 'edit'), when: canEdit },
    help: { page: 'Editing', text: 'edit it; enter again saves (the cover: choose a file)' },
  },
  {
    keys: ['d', 'Delete'],
    scope: 'inspector',
    action: 'clear',
    hint: { label: (s) => (onCover(s) ? 'remove' : 'clear'), when: (s) => onCover(s) || onCut(s) },
    help: { page: 'Editing', text: 'remove the cover (asks first); clear a cut, fade or volume' },
  },
  { keys: ['ArrowLeft'], scope: 'inspector', action: 'nudgeDown', hint: { label: 'nudge', when: onCut }, help: { page: 'Editing', text: 'nudge 0.1 s or 1.5 dB' } },
  { keys: ['ArrowRight'], scope: 'inspector', action: 'nudgeUp', hint: { label: 'nudge', when: onCut }, help: { page: 'Editing', text: 'nudge 0.1 s or 1.5 dB' } },
  { keys: ['Shift+ArrowLeft'], scope: 'inspector', action: 'nudgeDownMore', help: { page: 'Editing', text: 'nudge 1 s or 6 dB' } },
  { keys: ['Shift+ArrowRight'], scope: 'inspector', action: 'nudgeUpMore', help: { page: 'Editing', text: 'nudge 1 s or 6 dB' } },
  {
    keys: ['Space'],
    scope: 'inspector',
    action: 'hear',
    hint: { label: (s) => (s.playing ? 'stop' : 'hear'), when: onCut },
    help: { page: 'Editing', text: 'hear 8 s beside it; again, or esc, stops it' },
  },
  { keys: ['c'], scope: 'inspector', action: 'copyField', hint: { label: 'copy', when: onTrack }, help: { page: 'Editing', text: 'copy the selected field' } },

  { keys: ['Tab'], scope: 'common', action: 'toggleFocus', help: { page: 'Moving', text: 'the other pane' } },
  { keys: ['v'], scope: 'common', action: 'paste', hint: { label: 'paste', when: inInspector }, help: { page: 'Marking', text: 'paste them; ctrl+v with a picture sets the cover' } },
  { keys: ['/'], scope: 'common', action: 'focusFilter', hint: { label: 'filter', when: inTree }, help: { page: 'Moving', text: 'filter (esc clears it)' } },
  { keys: ['o'], scope: 'common', action: 'sortMenu', hint: { label: 'sort', when: inTree }, help: { page: 'Moving', text: 'sort the list' } },
  { keys: ['a'], scope: 'common', action: 'add', help: { page: 'Library', text: 'add from a YouTube link, or drop files on the page' } },
  { keys: ['s'], scope: 'common', action: 'sync', help: { page: 'Library', text: 'sync: write the folder' } },
  { keys: ['f'], scope: 'common', action: 'formats', help: { page: 'Library', text: 'formats: how each player is written; make, change, delete' } },
  { keys: ['u'], scope: 'common', action: 'undo', hint: { label: 'undo', when: (s) => Boolean(s.lib?.history.undo) }, help: { page: 'Library', text: 'undo / redo' } },
  { keys: ['Shift+u'], scope: 'common', action: 'redo', help: { page: 'Library', text: 'undo / redo' } },
  { keys: ['Shift+v'], scope: 'common', action: 'doctor', help: { page: 'Library', text: 'check the library' } },
  {
    keys: ['Escape'],
    scope: 'common',
    action: 'escape',
    hint: {
      label: (s) => (inInspector(s) ? 'back' : s.filter ? 'clear filter' : 'clear marks'),
      when: (s) => inInspector(s) || s.filter !== '' || s.marks.size > 0,
    },
    help: { page: 'Moving', text: 'back to the tree, or clear the filter, or the marks' },
  },
  { keys: ['?'], scope: 'common', action: 'help', hint: { label: 'help' }, help: { page: 'Library', text: 'this help' } },
];

const filterBindings: readonly { key: string; action: ActionName; hint?: string }[] = [
  { key: 'Enter', action: 'filterAccept', hint: 'keep' },
  { key: 'ArrowDown', action: 'filterAccept' },
  { key: 'Escape', action: 'filterClear', hint: 'clear' },
];

const labels: Record<string, string> = {
  ArrowUp: '↑',
  ArrowDown: '↓',
  ArrowLeft: '←',
  ArrowRight: '→',
  Enter: 'enter',
  Escape: 'esc',
  Tab: 'tab',
  Space: 'space',
  Delete: 'Del',
};

const arrows = new Set(['↑', '↓', '←', '→']);

const shifted = 'Shift+';

export function keyLabel(key: string): string {
  if (key.startsWith(shifted)) {
    const rest = key.slice(shifted.length);
    return /^[a-z]$/.test(rest) ? rest.toUpperCase() : `${shifted}${keyLabel(rest)}`;
  }
  return labels[key] ?? key;
}

function joinKeys(keys: readonly string[]): string {
  const shown = keys.map(keyLabel);
  if (shown.length > 1 && shown.every((k) => arrows.has(k))) {
    return shown.join('');
  }
  const digits = shown.map(Number);
  const consecutive = shown.every((k) => /^\d$/.test(k)) && digits.every((d, i) => i === 0 || d === (digits[i - 1] ?? 0) + 1);
  if (shown.length > 2 && consecutive) {
    return `${shown[0] ?? ''}-${shown[shown.length - 1] ?? ''}`;
  }
  return shown.join(' ');
}

export function keyName(e: KeyEventLike): string {
  const key = e.key === ' ' ? 'Space' : e.key;
  const letter = /^[a-zA-Z]$/.test(key);
  const shift = e.shiftKey && (letter || key.length > 1);
  return `${shift ? shifted : ''}${letter ? key.toLowerCase() : key}`;
}

function plain(e: KeyEventLike): boolean {
  return !e.isComposing && !e.ctrlKey && !e.metaKey && !e.altKey;
}

export function resolve(scope: 'tree' | 'inspector', e: KeyEventLike): ActionName | undefined {
  if (!plain(e)) {
    return undefined;
  }
  const name = keyName(e);
  const found = (s: Scope) => bindings.find((b) => b.scope === s && b.keys.includes(name));
  return (found(scope) ?? found('common'))?.action;
}

export function resolveFilter(e: KeyEventLike): ActionName | undefined {
  if (!plain(e)) {
    return undefined;
  }
  const name = keyName(e);
  return filterBindings.find((b) => b.key === name)?.action;
}

const editBindings: readonly { key: string; hint: string; second?: boolean }[] = [
  { key: 'Enter', hint: 'save' },
  { key: 'Tab', hint: 'next', second: true },
  { key: 'Escape', hint: 'cancel' },
];

export function editHints(second: boolean): readonly { keys: string; label: string }[] {
  return editBindings.flatMap((b) => (b.second === true && !second ? [] : [{ keys: keyLabel(b.key), label: b.hint }]));
}

export function filterHints(): readonly { keys: string; label: string }[] {
  return filterBindings.flatMap((b) => (b.hint === undefined ? [] : [{ keys: keyLabel(b.key), label: b.hint }]));
}

function grouped<T>(items: readonly T[], same: (a: T, b: T) => boolean): T[][] {
  const groups: T[][] = [];
  for (const item of items) {
    const last = groups[groups.length - 1];
    const head = last?.[0];
    if (last !== undefined && head !== undefined && same(head, item)) {
      last.push(item);
    } else {
      groups.push([item]);
    }
  }
  return groups;
}

export function hintsFor(s: AppState): readonly { keys: string; label: string }[] {
  if (s.overlay !== null || s.edit !== null) {
    return [];
  }
  const shown = bindings.flatMap((b) => {
    if ((b.scope !== s.focus && b.scope !== 'common') || !b.hint || !(b.hint.when?.(s) ?? true)) {
      return [];
    }
    const label = typeof b.hint.label === 'string' ? b.hint.label : b.hint.label(s);
    return [{ key: b.keys[0] ?? '', label }];
  });
  return grouped(shown, (a, b) => a.label === b.label).map((group) => ({
    keys: joinKeys(group.map((h) => h.key)),
    label: group[0]?.label ?? '',
  }));
}

const pages: readonly HelpPage[] = ['Moving', 'Editing', 'Marking', 'Library'];

function helpKeys(group: readonly Binding[]): string {
  const width = Math.max(...group.map((b) => b.keys.length));
  const columns = Array.from({ length: width }, (_, i) => joinKeys(group.flatMap((b) => b.keys[i] ?? [])));
  return columns.join('  ');
}

export function helpPages(): readonly { page: HelpPage; rows: readonly { keys: string; text: string }[] }[] {
  return pages.map((page) => {
    const helped = bindings.filter((b) => b.help.page === page);
    const rows = grouped(helped, (a, b) => a.help.text === b.help.text).map((group) => ({
      keys: helpKeys(group),
      text: group[0]?.help.text ?? '',
    }));
    return { page, rows };
  });
}
