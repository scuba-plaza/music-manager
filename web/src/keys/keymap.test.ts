import { describe, expect, it } from 'vitest';
import type { FieldView, PeekView, TrackView } from '../api/types.gen';
import { useApp, type AppState } from '../state/store';
import { libraryResponse } from '../testing/fakes';
import {
  bindings,
  editHints,
  filterHints,
  helpPages,
  hintsFor,
  keyLabel,
  keyName,
  resolve,
  type ActionName,
  type Binding,
  type KeyEventLike,
} from './keymap';

function press(key: string, mods: Partial<KeyEventLike> = {}): KeyEventLike {
  return { key, ctrlKey: false, metaKey: false, altKey: false, shiftKey: false, isComposing: false, ...mods };
}

function typed(name: string): KeyEventLike {
  const shift = name.startsWith('Shift+');
  const base = shift ? name.slice('Shift+'.length) : name;
  const key = base === 'Space' ? ' ' : shift && /^[a-z]$/.test(base) ? base.toUpperCase() : base;
  return press(key, { shiftKey: shift });
}

function state(overrides: Partial<AppState> = {}): AppState {
  return { ...useApp.getInitialState(), ...overrides };
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

const inTheTree: readonly [string, ActionName][] = [
  ['ArrowUp', 'moveUp'],
  ['k', 'moveUp'],
  ['ArrowDown', 'moveDown'],
  ['j', 'moveDown'],
  ['Home', 'moveToTop'],
  ['g', 'moveToTop'],
  ['End', 'moveToBottom'],
  ['Shift+g', 'moveToBottom'],
  ['Shift+ArrowRight', 'openLevel'],
  ['Shift+ArrowLeft', 'closeAll'],
  ['ArrowRight', 'open'],
  ['l', 'open'],
  ['Enter', 'enterRow'],
  ['ArrowLeft', 'close'],
  ['h', 'close'],
  ['Space', 'markToggle'],
  ['Shift+Space', 'markAll'],
  ['e', 'bulkEdit'],
  ['d', 'remove'],
  ['Delete', 'remove'],
  ['Shift+ArrowDown', 'moveTrackDown'],
  ['Shift+ArrowUp', 'moveTrackUp'],
  ['n', 'renumber'],
  ['p', 'addToPlaylist'],
  ['c', 'copyFrom'],
];

const inTheFields: readonly [string, ActionName][] = [
  ['ArrowUp', 'fieldUp'],
  ['k', 'fieldUp'],
  ['ArrowDown', 'fieldDown'],
  ['j', 'fieldDown'],
  ['1', 'tab1'],
  ['2', 'tab2'],
  ['3', 'tab3'],
  ['4', 'tab4'],
  ['Enter', 'startEdit'],
  ['d', 'clear'],
  ['Delete', 'clear'],
  ['ArrowLeft', 'nudgeDown'],
  ['ArrowRight', 'nudgeUp'],
  ['Shift+ArrowLeft', 'nudgeDownMore'],
  ['Shift+ArrowRight', 'nudgeUpMore'],
  ['Space', 'hear'],
  ['c', 'copyField'],
];

const inBoth: readonly [string, ActionName][] = [
  ['Tab', 'toggleFocus'],
  ['v', 'paste'],
  ['/', 'focusFilter'],
  ['o', 'sortMenu'],
  ['a', 'add'],
  ['s', 'sync'],
  ['f', 'formats'],
  ['u', 'undo'],
  ['Shift+u', 'redo'],
  ['Shift+v', 'doctor'],
  ['Escape', 'escape'],
  ['?', 'help'],
];

describe('keyName', () => {
  it('names a letter in lower case, with Shift in front of it only when Shift is held', () => {
    expect(keyName(press('j'))).toBe('j');
    expect(keyName(press('J', { shiftKey: true }))).toBe('Shift+j');
    expect(keyName(press('u', { shiftKey: true }))).toBe('Shift+u');
  });

  it('is not changed by Caps Lock, which makes a letter capital without Shift', () => {
    expect(keyName(press('U'))).toBe('u');
    expect(keyName(press('G'))).toBe('g');
  });

  it('names a symbol by what it types, so Shift is already in it', () => {
    expect(keyName(press('?', { shiftKey: true }))).toBe('?');
    expect(keyName(press('/'))).toBe('/');
  });

  it('names the space bar Space', () => {
    expect(keyName(press(' '))).toBe('Space');
    expect(keyName(press(' ', { shiftKey: true }))).toBe('Shift+Space');
  });

  it('names a named key by its name, with Shift in front of it when held', () => {
    expect(keyName(press('ArrowDown'))).toBe('ArrowDown');
    expect(keyName(press('ArrowRight', { shiftKey: true }))).toBe('Shift+ArrowRight');
    expect(keyName(press('Tab', { shiftKey: true }))).toBe('Shift+Tab');
    expect(keyName(press('Escape'))).toBe('Escape');
  });
});

describe('every key of the keymap', () => {
  it.each(inTheTree)('%s does %s in the tree', (key, action) => {
    expect(resolve('tree', typed(key))).toBe(action);
  });

  it.each(inTheFields)('%s does %s in the fields', (key, action) => {
    expect(resolve('inspector', typed(key))).toBe(action);
  });

  it.each(inBoth)('%s does %s in either pane', (key, action) => {
    expect(resolve('tree', typed(key))).toBe(action);
    expect(resolve('inspector', typed(key))).toBe(action);
  });

  it('is the whole table: no binding has a key the specification does not name', () => {
    const named = new Set([...inTheTree.map(([k]) => `tree ${k}`), ...inTheFields.map(([k]) => `inspector ${k}`), ...inBoth.map(([k]) => `common ${k}`)]);
    const bound = bindings.flatMap((b) => b.keys.map((k) => `${b.scope} ${k}`));
    expect(bound.filter((k) => !named.has(k))).toEqual([]);
    expect(new Set(bound).size).toBe(named.size);
  });
});

describe('what is gone', () => {
  const gone: [string, Partial<KeyEventLike>][] = [
    ['PageUp', {}],
    ['PageDown', {}],
    ['z', {}],
    ['+', {}],
    ['=', {}],
    ['-', {}],
    ['x', {}],
    ['0', {}],
    ['[', {}],
    [']', {}],
    ['{', { shiftKey: true }],
    ['}', { shiftKey: true }],
    ['i', {}],
    ['I', { shiftKey: true }],
    ['D', { shiftKey: true }],
    ['S', { shiftKey: true }],
    ['J', { shiftKey: true }],
    ['K', { shiftKey: true }],
    ['m', {}],
    ['t', {}],
    ['q', {}],
    ['Backspace', {}],
  ];

  it.each(gone)('%s does nothing in either pane', (key, mods) => {
    expect(resolve('tree', press(key, mods)), `tree ${key}`).toBeUndefined();
    expect(resolve('inspector', press(key, mods)), `inspector ${key}`).toBeUndefined();
  });

  it('keeps the browser its shortcuts: no key with ctrl, alt or meta held does anything', () => {
    for (const [key] of [...inTheTree, ...inTheFields, ...inBoth]) {
      for (const mods of [{ ctrlKey: true }, { altKey: true }, { metaKey: true }] as const) {
        expect(resolve('tree', { ...typed(key), ...mods }), `tree ${key} ${Object.keys(mods).join()}`).toBeUndefined();
        expect(resolve('inspector', { ...typed(key), ...mods }), `inspector ${key} ${Object.keys(mods).join()}`).toBeUndefined();
      }
    }
    expect(resolve('tree', press('r', { ctrlKey: true }))).toBeUndefined();
    expect(resolve('tree', press('a', { ctrlKey: true }))).toBeUndefined();
  });

  it('does nothing for a key that is being composed', () => {
    expect(resolve('tree', press('j', { isComposing: true }))).toBeUndefined();
  });
});

describe('the table', () => {
  it('no two bindings in one scope share a key', () => {
    for (const scope of ['tree', 'inspector'] as const) {
      const keys = bindings.filter((b) => b.scope === scope || b.scope === 'common').flatMap((b) => b.keys);
      expect(new Set(keys).size, scope).toBe(keys.length);
    }
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
      const { page, text } = b.help;
      const rows = helpPages().find((p) => p.page === page)?.rows ?? [];
      const row = rows.find((r) => r.text === text && b.keys.every((k) => shownKeys(r.keys).includes(keyLabel(k))));
      expect(row, `help of ${b.scope} ${b.action}`).toBeDefined();
    }
  });
});

describe('the key labels', () => {
  it('shows a shifted letter as a capital and a shifted named key with Shift in front', () => {
    expect(keyLabel('Shift+u')).toBe('U');
    expect(keyLabel('Shift+g')).toBe('G');
    expect(keyLabel('Shift+ArrowRight')).toBe('Shift+→');
    expect(keyLabel('Shift+Space')).toBe('Shift+space');
  });

  it('shows the named keys as the footer always has', () => {
    expect(keyLabel('Space')).toBe('space');
    expect(keyLabel('Delete')).toBe('Del');
    expect(keyLabel('Escape')).toBe('esc');
  });
});

describe('the footer hints', () => {
  it('the tree shows its keys in the order of the table', () => {
    expect(hintsFor(state())).toEqual([
      { keys: '↑↓', label: 'move' },
      { keys: '→', label: 'fields' },
      { keys: 'space', label: 'mark' },
      { keys: 'e', label: 'edit' },
      { keys: 'd', label: 'remove' },
      { keys: 'p', label: 'playlist' },
      { keys: '/', label: 'filter' },
      { keys: 'o', label: 'sort' },
      { keys: '?', label: 'help' },
    ]);
  });

  it('u undo shows only when there is something to undo', () => {
    const labels = (s: AppState) => hintsFor(s).map((h) => `${h.keys} ${h.label}`);
    expect(labels(state())).not.toContain('u undo');
    expect(labels(state({ lib: libraryResponse() }))).not.toContain('u undo');
    expect(labels(state({ lib: undoable }))).toContain('u undo');
    expect(labels(state({ lib: undoable })).indexOf('u undo')).toBe(labels(state({ lib: undoable })).indexOf('o sort') + 1);
  });

  it('esc in the tree clears the filter before the marks, and shows only when there is one of them', () => {
    const esc = (s: AppState) => hintsFor(s).find((h) => h.keys === 'esc')?.label;
    expect(esc(state())).toBeUndefined();
    expect(esc(state({ marks: new Set(['t1']) }))).toBe('clear marks');
    expect(esc(state({ filter: 'bach', marks: new Set(['t1']) }))).toBe('clear filter');
  });

  it('the fields on a cut of a track show the clear, nudge and hear keys', () => {
    expect(hintsFor(state({ focus: 'inspector', tab: 1, detail: track([], [field('cutStart')]) }))).toEqual([
      { keys: '↑↓', label: 'field' },
      { keys: '1-4', label: 'tabs' },
      { keys: 'enter', label: 'edit' },
      { keys: 'd', label: 'clear' },
      { keys: '←→', label: 'nudge' },
      { keys: 'space', label: 'hear' },
      { keys: 'c', label: 'copy' },
      { keys: 'v', label: 'paste' },
      { keys: 'esc', label: 'back' },
      { keys: '?', label: 'help' },
    ]);
  });

  it('space says stop while something plays', () => {
    const s = state({ focus: 'inspector', tab: 1, detail: track([], [field('fadeOut')]), playing: { id: 't1', field: 'fadeOut' } });
    expect(hintsFor(s)).toContainEqual({ keys: 'space', label: 'stop' });
  });

  it('the cover field offers enter to choose a file and d to remove it', () => {
    const hints = hintsFor(state({ focus: 'inspector', detail: track([field('cover')]) }));
    expect(hints.slice(2, 4)).toEqual([
      { keys: 'enter', label: 'choose' },
      { keys: 'd', label: 'remove' },
    ]);
    expect(hints).not.toContainEqual({ keys: 'enter', label: 'edit' });
    expect(hints.map((h) => h.label)).not.toContain('nudge');
  });

  it('enter edit shows only on a field that can be edited, and d clear only where something can be cleared', () => {
    const enter = (f: FieldView) => hintsFor(state({ focus: 'inspector', detail: track([f]) })).find((h) => h.keys === 'enter');
    expect(enter(field('title'))).toEqual({ keys: 'enter', label: 'edit' });
    expect(enter(field('titleLatin', false))).toBeUndefined();
    expect(hintsFor(state({ focus: 'inspector', detail: track([field('title')]) })).find((h) => h.keys === 'd')).toBeUndefined();
  });

  it('the Device tab, which has no fields, hints the tabs but nothing to edit', () => {
    const hints = hintsFor(state({ focus: 'inspector', tab: 1, detail: track([field('title')], []) }));
    expect(hints).toContainEqual({ keys: '1-4', label: 'tabs' });
    expect(hints.find((h) => h.keys === 'enter')).toBeUndefined();
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

  it.each([
    ['Library', 'u U', 'undo / redo'],
    ['Moving', '↑↓  k j', 'move'],
    ['Moving', 'Home End  g G', 'top / bottom'],
    ['Editing', '1-4', "a track's tab by its number: Details, Audio, Latin, Device"],
    ['Editing', '←→', 'nudge 0.1 s or 1.5 dB'],
    ['Editing', 'Shift+← Shift+→', 'nudge 1 s or 6 dB'],
    ['Editing', 'd  Del', 'remove the cover (asks first); clear a cut, fade or volume'],
  ])('%s joins the keys of neighbouring bindings that say the same thing: %s', (page, keys, text) => {
    expect(helpPages().find((p) => p.page === page)?.rows).toContainEqual({ keys, text });
  });
});

describe('the filter box', () => {
  it('hints enter keep and esc clear', () => {
    expect(filterHints()).toEqual([
      { keys: 'enter', label: 'keep' },
      { keys: 'esc', label: 'clear' },
    ]);
  });
});

describe('the keys of a field being typed in', () => {
  it('hints enter save and esc cancel', () => {
    expect(editHints(false)).toEqual([
      { keys: 'enter', label: 'save' },
      { keys: 'esc', label: 'cancel' },
    ]);
  });

  it('hints tab next between them for the two numbers of a track', () => {
    expect(editHints(true)).toEqual([
      { keys: 'enter', label: 'save' },
      { keys: 'tab', label: 'next' },
      { keys: 'esc', label: 'cancel' },
    ]);
  });
});
