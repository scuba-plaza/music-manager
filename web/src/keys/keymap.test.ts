import { describe, expect, it } from 'vitest';
import type { FieldView, PeekView, TrackView } from '../api/types.gen';
import type { AppState } from '../state/store';
import { libraryResponse } from '../testing/fakes';
import {
  bindings,
  filterHints,
  helpPages,
  hintsFor,
  keyLabel,
  keyName,
  resolve,
  resolveFilter,
  type Binding,
  type KeyEventLike,
} from './keymap';

function press(key: string, mods: Partial<KeyEventLike> = {}): KeyEventLike {
  return { key, ctrlKey: false, metaKey: false, altKey: false, shiftKey: false, isComposing: false, ...mods };
}

function state(overrides: Partial<AppState> = {}): AppState {
  return {
    info: null,
    lib: null,
    filter: '',
    sort: { key: '', reverse: false },
    expanded: new Set(),
    cursor: null,
    marks: new Set(),
    focus: 'tree',
    field: 0,
    tab: 0,
    detail: null,
    edit: null,
    playing: null,
    status: { text: '', kind: '' },
    overlay: null,
    busy: false,
    ...overrides,
  };
}

function field(key: string, editable = true): FieldView {
  return { key, label: key, text: '', editable, edit: '' };
}

function track(fields: FieldView[], audio: FieldView[] = []): TrackView {
  return {
    id: 't1',
    title: 'A track',
    identified: true,
    tabs: [
      { id: 'details', name: 'Details', fields },
      { id: 'audio', name: 'Audio', fields: audio },
    ],
    path: '',
    dates: '',
    about: '',
    reveal: [],
  };
}

function artist(fields: FieldView[]): PeekView {
  return { kind: 'artist', title: 'An artist', label: 'Artist', editable: true, info: [], fields, list: [] };
}

const arrows = ['↑', '↓', '←', '→'];

function shownKeys(text: string): string[] {
  return text.split(/\s+/).flatMap((token) => {
    const range = /^(\d)-(\d)$/.exec(token);
    if (range) {
      const from = Number(range[1]);
      const to = Number(range[2]);
      return Array.from({ length: to - from + 1 }, (_, i) => String(from + i));
    }
    if (token !== '' && Array.from(token).every((c) => arrows.includes(c))) {
      return Array.from(token);
    }
    return [token];
  });
}

function labelOf(b: Binding, s: AppState): string | undefined {
  if (!b.hint) {
    return undefined;
  }
  return typeof b.hint.label === 'string' ? b.hint.label : b.hint.label(s);
}

const undoable = libraryResponse('s1', { history: { undo: 'edit' } });

const samples: AppState[] = [
  state({ lib: undoable, filter: 'bach' }),
  state({ marks: new Set(['t1']) }),
  state({ focus: 'inspector', lib: undoable, detail: track([field('cover')]) }),
  state({ focus: 'inspector', detail: track([field('title')]) }),
  state({ focus: 'inspector', tab: 1, detail: track([], [field('cutStart')]) }),
  state({ focus: 'inspector', tab: 1, detail: track([], [field('cutStart')]), playing: { id: 't1', field: 'cutStart' } }),
];

describe('keyName', () => {
  it('names a printable key by what it types, so shift is already in it', () => {
    expect(keyName(press('j'))).toBe('j');
    expect(keyName(press('J', { shiftKey: true }))).toBe('J');
    expect(keyName(press('{', { shiftKey: true }))).toBe('{');
    expect(keyName(press('?', { shiftKey: true }))).toBe('?');
  });

  it('names the space bar space', () => {
    expect(keyName(press(' '))).toBe('space');
  });

  it('names a named key by its name and leaves shift out of it', () => {
    expect(keyName(press('ArrowDown'))).toBe('ArrowDown');
    expect(keyName(press('PageUp'))).toBe('PageUp');
    expect(keyName(press('Tab', { shiftKey: true }))).toBe('Tab');
    expect(keyName(press('Escape'))).toBe('Escape');
  });

  it('puts ctrl before the lower-cased key and leaves shift out of it', () => {
    expect(keyName(press('r', { ctrlKey: true }))).toBe('ctrl+r');
    expect(keyName(press('R', { ctrlKey: true, shiftKey: true }))).toBe('ctrl+r');
    expect(keyName(press(' ', { ctrlKey: true }))).toBe('ctrl+space');
  });

  it('names alt and meta too, so they are never taken for the plain key', () => {
    expect(keyName(press('j', { altKey: true }))).toBe('alt+j');
    expect(keyName(press('R', { metaKey: true }))).toBe('meta+r');
    expect(keyName(press('r', { ctrlKey: true, altKey: true, metaKey: true }))).toBe('ctrl+alt+meta+r');
  });
});

describe('resolve', () => {
  it('resolve picks the tree binding in the tree and the inspector binding in the inspector for p', () => {
    expect(resolve('tree', press('p'))).toBe('addToPlaylist');
    expect(resolve('inspector', press('p'))).toBe('hear');
  });

  it('common bindings work in both scopes', () => {
    for (const b of bindings.filter((binding) => binding.scope === 'common')) {
      for (const key of b.keys) {
        expect(resolve('tree', press(key === 'space' ? ' ' : key)), key).toBe(b.action);
        expect(resolve('inspector', press(key === 'space' ? ' ' : key)), key).toBe(b.action);
      }
    }
    expect(resolve('tree', press('?'))).toBe('help');
    expect(resolve('inspector', press('u'))).toBe('undo');
  });

  it('a key of one pane does nothing in the other', () => {
    expect(resolve('inspector', press('J'))).toBeUndefined();
    expect(resolve('inspector', press('u', { ctrlKey: true }))).toBeUndefined();
    expect(resolve('inspector', press(' '))).toBeUndefined();
    expect(resolve('tree', press('x'))).toBeUndefined();
    expect(resolve('tree', press('1'))).toBeUndefined();
    expect(resolve('tree', press('['))).toBeUndefined();
  });

  it('the arrows, enter and the vim keys mean one thing in the tree and another in the inspector', () => {
    expect(resolve('tree', press('ArrowDown'))).toBe('moveDown');
    expect(resolve('tree', press('j'))).toBe('moveDown');
    expect(resolve('inspector', press('ArrowDown'))).toBe('fieldDown');
    expect(resolve('inspector', press('k'))).toBe('fieldUp');
    expect(resolve('tree', press('ArrowRight'))).toBe('open');
    expect(resolve('inspector', press('l'))).toBe('nextTab');
    expect(resolve('tree', press('h'))).toBe('close');
    expect(resolve('inspector', press('ArrowLeft'))).toBe('previousTabOrBack');
    expect(resolve('tree', press('Enter'))).toBe('enterRow');
    expect(resolve('inspector', press('Enter'))).toBe('startEdit');
  });

  it('resolves nothing for meta, alt or a key being composed', () => {
    expect(resolve('tree', press('j', { metaKey: true }))).toBeUndefined();
    expect(resolve('tree', press('j', { altKey: true }))).toBeUndefined();
    expect(resolve('tree', press('r', { ctrlKey: true, metaKey: true }))).toBeUndefined();
    expect(resolve('tree', press('j', { isComposing: true }))).toBeUndefined();
  });

  it('resolves a ctrl key only by its ctrl binding', () => {
    expect(resolve('tree', press('a', { ctrlKey: true }))).toBe('markAll');
    expect(resolve('tree', press('c', { ctrlKey: true }))).toBeUndefined();
    expect(resolve('tree', press('j', { ctrlKey: true }))).toBeUndefined();
  });

  it('every key of the README table has a binding', () => {
    const readme: [('tree' | 'inspector'), string, Partial<KeyEventLike>?][] = [
      ['tree', 'ArrowUp'], ['tree', 'ArrowDown'], ['tree', 'j'], ['tree', 'k'],
      ['tree', 'PageUp'], ['tree', 'PageDown'], ['tree', 'u', { ctrlKey: true }], ['tree', 'd', { ctrlKey: true }],
      ['tree', 'Home'], ['tree', 'g'], ['tree', 'End'], ['tree', 'G'],
      ['tree', 'z'],
      ['tree', 'ArrowRight'], ['tree', 'l'], ['tree', 'ArrowLeft'], ['tree', 'h'],
      ['inspector', 'ArrowRight'], ['inspector', 'l'], ['inspector', 'ArrowLeft'], ['inspector', 'h'],
      ['tree', 'Enter'], ['inspector', 'Enter'],
      ['tree', '+'], ['tree', '='], ['tree', '-'],
      ['tree', ' '], ['tree', 'a', { ctrlKey: true }],
      ['tree', 'e'], ['tree', 'c'], ['tree', 'v'], ['tree', 'd'], ['tree', 'Delete'],
      ['tree', 'J'], ['tree', 'K'], ['tree', 'n'], ['tree', 'p'], ['inspector', 'p'],
      ['tree', 'i'], ['tree', 'I'], ['tree', 'o'],
      ['tree', 'a'], ['tree', 's'], ['tree', 'D'],
      ['tree', 'u'], ['tree', 'r', { ctrlKey: true }],
      ['inspector', '['], ['inspector', ']'], ['inspector', '{'], ['inspector', '}'], ['inspector', '0'],
      ['inspector', 'x'],
      ['inspector', '1'], ['inspector', '2'], ['inspector', '3'], ['inspector', '4'],
      ['tree', '/'], ['tree', 'S'], ['tree', '?'], ['tree', 'Tab'], ['tree', 'Escape'],
    ];
    for (const [scope, key, mods] of readme) {
      expect(resolve(scope, press(key, mods)), `${scope} ${key}`).toBeDefined();
    }
  });
});

describe('the table', () => {
  it('no two bindings in one scope share a key', () => {
    for (const scope of ['tree', 'inspector'] as const) {
      const keys = bindings.filter((b) => b.scope === scope || b.scope === 'common').flatMap((b) => b.keys);
      expect(new Set(keys).size, scope).toBe(keys.length);
    }
    const common = bindings.filter((b) => b.scope === 'common').flatMap((b) => b.keys);
    expect(new Set(common).size).toBe(common.length);
  });

  it('has one entry for each action of a scope', () => {
    const pairs = bindings.map((b) => `${b.scope} ${b.action}`);
    expect(new Set(pairs).size).toBe(pairs.length);
  });

  it('every binding has a line in the help, so ? lists every key', () => {
    expect(bindings.filter((b) => !b.help).map((b) => b.action)).toEqual([]);
  });

  it('every binding with a hint or help appears in hintsFor / helpPages', () => {
    for (const b of bindings) {
      if (b.hint) {
        const first = keyLabel(b.keys[0] ?? '');
        const found = samples.some((s) =>
          hintsFor(s).some((h) => h.label === labelOf(b, s) && shownKeys(h.keys).includes(first)),
        );
        expect(found, `hint of ${b.scope} ${b.action}`).toBe(true);
      }
      if (b.help) {
        const { page, text } = b.help;
        const rows = helpPages().find((p) => p.page === page)?.rows ?? [];
        const row = rows.find((r) => r.text === text && b.keys.every((k) => shownKeys(r.keys).includes(keyLabel(k))));
        expect(row, `help of ${b.scope} ${b.action}`).toBeDefined();
      }
    }
  });
});

describe('the footer hints', () => {
  it('the tree shows the keys of the old footer, in its order', () => {
    expect(hintsFor(state())).toEqual([
      { keys: '↑↓', label: 'move' },
      { keys: '→', label: 'fields' },
      { keys: 'space', label: 'mark' },
      { keys: 'e', label: 'edit' },
      { keys: 'd', label: 'remove' },
      { keys: 'p', label: 'playlist' },
      { keys: '/', label: 'filter' },
      { keys: 'S', label: 'sort' },
      { keys: '?', label: 'help' },
    ]);
  });

  it('u undo shows only when there is something to undo', () => {
    const labels = (s: AppState) => hintsFor(s).map((h) => `${h.keys} ${h.label}`);
    expect(labels(state())).not.toContain('u undo');
    expect(labels(state({ lib: libraryResponse() }))).not.toContain('u undo');
    expect(labels(state({ lib: undoable }))).toContain('u undo');
    expect(labels(state({ lib: undoable })).indexOf('u undo')).toBe(labels(state({ lib: undoable })).indexOf('S sort') + 1);
  });

  it('esc in the tree clears the filter before the marks, and shows only when there is one of them', () => {
    const esc = (s: AppState) => hintsFor(s).find((h) => h.keys === 'esc')?.label;
    expect(esc(state())).toBeUndefined();
    expect(esc(state({ marks: new Set(['t1']) }))).toBe('clear marks');
    expect(esc(state({ filter: 'bach', marks: new Set(['t1']) }))).toBe('clear filter');
  });

  it('the inspector on a cut of a track shows the nudge, clear and hear keys', () => {
    expect(hintsFor(state({ focus: 'inspector', tab: 1, detail: track([], [field('cutStart')]) }))).toEqual([
      { keys: '↑↓', label: 'field' },
      { keys: '1-4', label: 'tabs' },
      { keys: 'enter', label: 'edit' },
      { keys: '[ ]', label: 'nudge' },
      { keys: '0', label: 'clear' },
      { keys: 'p', label: 'hear' },
      { keys: 'c', label: 'copy' },
      { keys: 'v', label: 'paste' },
      { keys: 'esc', label: 'back' },
      { keys: '?', label: 'help' },
    ]);
  });

  it('p says stop while something plays', () => {
    const s = state({ focus: 'inspector', tab: 1, detail: track([], [field('fadeOut')]), playing: { id: 't1', field: 'fadeOut' } });
    expect(hintsFor(s)).toContainEqual({ keys: 'p', label: 'stop' });
  });

  it('the cover field is chosen with enter and removed with x', () => {
    const hints = hintsFor(state({ focus: 'inspector', detail: track([field('cover')]) }));
    expect(hints).toContainEqual({ keys: 'enter', label: 'choose' });
    expect(hints).toContainEqual({ keys: 'x', label: 'remove' });
    expect(hints).not.toContainEqual({ keys: 'enter', label: 'edit' });
    expect(hints.map((h) => h.label)).not.toContain('nudge');
  });

  it('enter edit shows only on a field that can be edited, and never on the Wolfgang', () => {
    const enter = (f: FieldView) => hintsFor(state({ focus: 'inspector', detail: track([f]) })).find((h) => h.keys === 'enter');
    expect(enter(field('title'))).toEqual({ keys: 'enter', label: 'edit' });
    expect(enter(field('titleLatin', false))).toBeUndefined();
    expect(enter(field('wolfgang'))).toBeUndefined();
  });

  it('the tab shown is kept within the tabs the track has', () => {
    const hints = hintsFor(state({ focus: 'inspector', tab: 7, detail: track([], [field('volume')]) }));
    expect(hints).toContainEqual({ keys: '[ ]', label: 'nudge' });
  });

  it('the fields of an artist have no tabs and nothing to copy', () => {
    expect(hintsFor(state({ focus: 'inspector', detail: artist([field('name')]) }))).toEqual([
      { keys: '↑↓', label: 'field' },
      { keys: 'enter', label: 'edit' },
      { keys: 'v', label: 'paste' },
      { keys: 'esc', label: 'back' },
      { keys: '?', label: 'help' },
    ]);
  });

  it('no pane hints while a dialog is open or a field is being typed in', () => {
    expect(hintsFor(state({ overlay: { kind: 'help' } }))).toEqual([]);
    expect(hintsFor(state({ focus: 'inspector', detail: track([field('title')]), edit: { field: 0, text: '' } }))).toEqual([]);
  });
});

describe('the help pages', () => {
  it('come in the order of the old help', () => {
    expect(helpPages().map((p) => p.page)).toEqual(['Moving', 'Editing', 'Marking', 'Library']);
  });

  it('a row joins the keys of neighbouring bindings that say the same thing', () => {
    const library = helpPages().find((p) => p.page === 'Library')?.rows ?? [];
    expect(library).toContainEqual({ keys: 'u ctrl+r', text: 'undo / redo' });
    const moving = helpPages().find((p) => p.page === 'Moving')?.rows ?? [];
    expect(moving).toContainEqual({ keys: '↑↓  k j', text: 'move' });
    const editing = helpPages().find((p) => p.page === 'Editing')?.rows ?? [];
    expect(editing.find((r) => r.keys === '1-4')).toBeDefined();
  });
});

describe('the filter box', () => {
  it('takes enter and down into the tree and clears with escape, and nothing else', () => {
    expect(resolveFilter(press('Enter'))).toBe('filterAccept');
    expect(resolveFilter(press('ArrowDown'))).toBe('filterAccept');
    expect(resolveFilter(press('Escape'))).toBe('filterClear');
    expect(resolveFilter(press('j'))).toBeUndefined();
    expect(resolveFilter(press('ArrowUp'))).toBeUndefined();
    expect(resolveFilter(press('a', { ctrlKey: true }))).toBeUndefined();
  });

  it('hints enter keep and esc clear', () => {
    expect(filterHints()).toEqual([
      { keys: 'enter', label: 'keep' },
      { keys: 'esc', label: 'clear' },
    ]);
  });
});
