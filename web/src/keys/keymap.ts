import type { FieldView } from '../api/types.gen';
import type { AppState } from '../state/store';

export type Scope = 'common' | 'tree' | 'inspector';

export type KeyEventLike = Pick<KeyboardEvent, 'key' | 'ctrlKey' | 'metaKey' | 'altKey' | 'shiftKey' | 'isComposing'>;

export type HelpPage = 'Moving' | 'Editing' | 'Marking' | 'Library';

export type ActionName =
  | 'moveUp'
  | 'moveDown'
  | 'pageUp'
  | 'pageDown'
  | 'moveToTop'
  | 'moveToBottom'
  | 'centre'
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
  | 'identifyAll'
  | 'fieldUp'
  | 'fieldDown'
  | 'previousTabOrBack'
  | 'nextTab'
  | 'tab1'
  | 'tab2'
  | 'tab3'
  | 'tab4'
  | 'startEdit'
  | 'removeCover'
  | 'nudgeDown'
  | 'nudgeUp'
  | 'nudgeDownMore'
  | 'nudgeUpMore'
  | 'clearField'
  | 'hear'
  | 'toggleFocus'
  | 'focusFilter'
  | 'sortMenu'
  | 'copy'
  | 'paste'
  | 'identify'
  | 'adopt'
  | 'add'
  | 'sync'
  | 'doctor'
  | 'undo'
  | 'redo'
  | 'escape'
  | 'help'
  | 'filterAccept'
  | 'filterClear';

export interface Binding {
  keys: readonly string[];
  scope: Scope;
  action: ActionName;
  hint?: { label: string | ((s: AppState) => string); when?: (s: AppState) => boolean };
  help?: { page: HelpPage; text: string };
}

const cutFields = ['cutStart', 'cutEnd', 'volume', 'fadeIn', 'fadeOut'];

function onTrack(s: AppState): boolean {
  return s.detail !== null && 'tabs' in s.detail;
}

function currentField(s: AppState): FieldView | undefined {
  const d = s.detail;
  if (s.focus !== 'inspector' || d === null) {
    return undefined;
  }
  if ('tabs' in d) {
    return d.tabs[Math.min(s.tab, d.tabs.length - 1)]?.fields[s.field];
  }
  return d.fields?.[s.field];
}

function onCover(s: AppState): boolean {
  return currentField(s)?.key === 'cover';
}

function onCut(s: AppState): boolean {
  return cutFields.includes(currentField(s)?.key ?? '');
}

function canEdit(s: AppState): boolean {
  return currentField(s)?.editable === true;
}

const tabs = { label: 'tabs', when: onTrack };
const inTree = (s: AppState) => s.focus === 'tree';
const inInspector = (s: AppState) => s.focus === 'inspector';

export const bindings: readonly Binding[] = [
  { keys: ['ArrowUp', 'k'], scope: 'tree', action: 'moveUp', hint: { label: 'move' }, help: { page: 'Moving', text: 'move' } },
  { keys: ['ArrowDown', 'j'], scope: 'tree', action: 'moveDown', hint: { label: 'move' }, help: { page: 'Moving', text: 'move' } },
  { keys: ['PageUp', 'ctrl+u'], scope: 'tree', action: 'pageUp', help: { page: 'Moving', text: 'a page at a time' } },
  { keys: ['PageDown', 'ctrl+d'], scope: 'tree', action: 'pageDown', help: { page: 'Moving', text: 'a page at a time' } },
  { keys: ['Home', 'g'], scope: 'tree', action: 'moveToTop', help: { page: 'Moving', text: 'top / bottom' } },
  { keys: ['End', 'G'], scope: 'tree', action: 'moveToBottom', help: { page: 'Moving', text: 'top / bottom' } },
  { keys: ['z'], scope: 'tree', action: 'centre', help: { page: 'Moving', text: 'centre the cursor' } },
  { keys: ['+', '='], scope: 'tree', action: 'openLevel', help: { page: 'Moving', text: 'open a level' } },
  { keys: ['-'], scope: 'tree', action: 'closeAll', help: { page: 'Moving', text: 'close all' } },
  {
    keys: ['ArrowRight', 'l'],
    scope: 'tree',
    action: 'open',
    hint: { label: 'fields' },
    help: { page: 'Moving', text: 'into the fields of a track, artist, album or playlist' },
  },
  { keys: ['Enter'], scope: 'tree', action: 'enterRow', help: { page: 'Moving', text: 'open / close; on a track, into its fields' } },
  { keys: ['ArrowLeft', 'h'], scope: 'tree', action: 'close', help: { page: 'Moving', text: 'close, or up a level' } },
  { keys: ['space'], scope: 'tree', action: 'markToggle', hint: { label: 'mark' }, help: { page: 'Marking', text: 'mark a track (an album marks all its tracks)' } },
  { keys: ['ctrl+a'], scope: 'tree', action: 'markAll', help: { page: 'Marking', text: 'mark everything shown' } },
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
  { keys: ['J'], scope: 'tree', action: 'moveTrackDown', help: { page: 'Marking', text: 'move a track down / up' } },
  { keys: ['K'], scope: 'tree', action: 'moveTrackUp', help: { page: 'Marking', text: 'move a track down / up' } },
  { keys: ['n'], scope: 'tree', action: 'renumber', help: { page: 'Marking', text: 'number an album 1…N' } },
  { keys: ['p'], scope: 'tree', action: 'addToPlaylist', hint: { label: 'playlist' }, help: { page: 'Marking', text: 'add to a playlist' } },
  { keys: ['I'], scope: 'tree', action: 'identifyAll', help: { page: 'Library', text: 'identify everything' } },

  { keys: ['ArrowUp', 'k'], scope: 'inspector', action: 'fieldUp', hint: { label: 'field' }, help: { page: 'Editing', text: 'choose a field' } },
  { keys: ['ArrowDown', 'j'], scope: 'inspector', action: 'fieldDown', hint: { label: 'field' }, help: { page: 'Editing', text: 'choose a field' } },
  {
    keys: ['ArrowLeft', 'h'],
    scope: 'inspector',
    action: 'previousTabOrBack',
    help: { page: 'Editing', text: "a track's tabs: Details, Audio, Latin, Device; ← on the first goes back to the tree" },
  },
  {
    keys: ['ArrowRight', 'l'],
    scope: 'inspector',
    action: 'nextTab',
    help: { page: 'Editing', text: "a track's tabs: Details, Audio, Latin, Device; ← on the first goes back to the tree" },
  },
  { keys: ['1'], scope: 'inspector', action: 'tab1', hint: tabs, help: { page: 'Editing', text: "a track's tab by its number (4: Device, when a device profile is chosen)" } },
  { keys: ['2'], scope: 'inspector', action: 'tab2', hint: tabs, help: { page: 'Editing', text: "a track's tab by its number (4: Device, when a device profile is chosen)" } },
  { keys: ['3'], scope: 'inspector', action: 'tab3', hint: tabs, help: { page: 'Editing', text: "a track's tab by its number (4: Device, when a device profile is chosen)" } },
  { keys: ['4'], scope: 'inspector', action: 'tab4', hint: tabs, help: { page: 'Editing', text: "a track's tab by its number (4: Device, when a device profile is chosen)" } },
  {
    keys: ['Enter'],
    scope: 'inspector',
    action: 'startEdit',
    hint: { label: (s) => (onCover(s) ? 'choose' : 'edit'), when: (s) => onCover(s) || canEdit(s) },
    help: { page: 'Editing', text: 'edit it; enter again saves (the cover: choose a file)' },
  },
  { keys: ['x'], scope: 'inspector', action: 'removeCover', hint: { label: 'remove', when: onCover }, help: { page: 'Editing', text: 'remove the cover' } },
  { keys: ['['], scope: 'inspector', action: 'nudgeDown', hint: { label: 'nudge', when: onCut }, help: { page: 'Editing', text: 'nudge 0.1 s or 1.5 dB' } },
  { keys: [']'], scope: 'inspector', action: 'nudgeUp', hint: { label: 'nudge', when: onCut }, help: { page: 'Editing', text: 'nudge 0.1 s or 1.5 dB' } },
  { keys: ['{'], scope: 'inspector', action: 'nudgeDownMore', help: { page: 'Editing', text: 'nudge 1 s or 6 dB' } },
  { keys: ['}'], scope: 'inspector', action: 'nudgeUpMore', help: { page: 'Editing', text: 'nudge 1 s or 6 dB' } },
  { keys: ['0'], scope: 'inspector', action: 'clearField', hint: { label: 'clear', when: onCut }, help: { page: 'Editing', text: 'no cut, fade or change' } },
  {
    keys: ['p'],
    scope: 'inspector',
    action: 'hear',
    hint: { label: (s) => (s.playing ? 'stop' : 'hear'), when: onCut },
    help: { page: 'Editing', text: 'hear 8 s beside it' },
  },

  { keys: ['Tab'], scope: 'common', action: 'toggleFocus', help: { page: 'Moving', text: 'the other pane' } },
  { keys: ['c'], scope: 'common', action: 'copy', hint: { label: 'copy', when: (s) => inInspector(s) && onTrack(s) }, help: { page: 'Marking', text: 'copy the cover, artist, album… of a track' } },
  { keys: ['v'], scope: 'common', action: 'paste', hint: { label: 'paste', when: inInspector }, help: { page: 'Marking', text: 'paste them; ctrl+v with a picture sets the cover' } },
  { keys: ['/'], scope: 'common', action: 'focusFilter', hint: { label: 'filter', when: inTree }, help: { page: 'Moving', text: 'filter (esc clears it)' } },
  { keys: ['S'], scope: 'common', action: 'sortMenu', hint: { label: 'sort', when: inTree }, help: { page: 'Moving', text: 'sort the list' } },
  { keys: ['a'], scope: 'common', action: 'add', help: { page: 'Library', text: 'add from a YouTube link, or drop files on the page' } },
  { keys: ['i'], scope: 'common', action: 'identify', help: { page: 'Library', text: 'find it on MusicBrainz' } },
  { keys: ['o'], scope: 'common', action: 'adopt', help: { page: 'Library', text: "use the file's old tags" } },
  { keys: ['s'], scope: 'common', action: 'sync', help: { page: 'Library', text: 'sync: write the folder' } },
  { keys: ['u'], scope: 'common', action: 'undo', hint: { label: 'undo', when: (s) => Boolean(s.lib?.history.undo) }, help: { page: 'Library', text: 'undo / redo' } },
  { keys: ['ctrl+r'], scope: 'common', action: 'redo', help: { page: 'Library', text: 'undo / redo' } },
  { keys: ['D'], scope: 'common', action: 'doctor', help: { page: 'Library', text: 'check the library' } },
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
  PageUp: 'PgUp',
  PageDown: 'PgDn',
  Delete: 'Del',
};

const arrows = new Set(['↑', '↓', '←', '→']);

export function keyLabel(key: string): string {
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

const modifiers = [
  ['ctrl', 'ctrlKey'],
  ['alt', 'altKey'],
  ['meta', 'metaKey'],
] as const;

export function keyName(e: KeyEventLike): string {
  const key = e.key === ' ' ? 'space' : e.key;
  const held = modifiers.filter(([, flag]) => e[flag]).map(([name]) => name);
  return held.length === 0 ? key : `${held.join('+')}+${key.toLowerCase()}`;
}

function usable(e: KeyEventLike): boolean {
  return !e.metaKey && !e.altKey && !e.isComposing;
}

export function resolve(scope: 'tree' | 'inspector', e: KeyEventLike): ActionName | undefined {
  if (!usable(e)) {
    return undefined;
  }
  const name = keyName(e);
  const found = (s: Scope) => bindings.find((b) => b.scope === s && b.keys.includes(name));
  return (found(scope) ?? found('common'))?.action;
}

export function resolveFilter(e: KeyEventLike): ActionName | undefined {
  if (!usable(e)) {
    return undefined;
  }
  const name = keyName(e);
  return filterBindings.find((b) => b.key === name)?.action;
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
    const helped = bindings.filter((b) => b.help?.page === page);
    const rows = grouped(helped, (a, b) => a.help?.text === b.help?.text).map((group) => ({
      keys: helpKeys(group),
      text: group[0]?.help?.text ?? '',
    }));
    return { page, rows };
  });
}
