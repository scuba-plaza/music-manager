import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { memoryStorage } from '../testing/fakes';
import type { ActionName } from './keymap';

interface Pressed {
  event: KeyboardEvent;
  prevented: () => boolean;
}

interface Target {
  tagName: string;
  hasAttribute?: (name: string) => boolean;
}

const body: Target = { tagName: 'BODY' };

function press(key: string, mods: { ctrl?: boolean; meta?: boolean; alt?: boolean; shift?: boolean; composing?: boolean } = {}, target: Target = body): Pressed {
  let prevented = false;
  const event = {
    key,
    ctrlKey: mods.ctrl ?? false,
    metaKey: mods.meta ?? false,
    altKey: mods.alt ?? false,
    shiftKey: mods.shift ?? false,
    isComposing: mods.composing ?? false,
    target,
    preventDefault: () => {
      prevented = true;
    },
  };
  return { event: event as unknown as KeyboardEvent, prevented: () => prevented };
}

async function setup() {
  vi.resetModules();
  vi.stubGlobal('localStorage', memoryStorage());
  const { installKeys, filterBoxAttribute } = await import('./dispatch');
  const { useApp } = await import('../state/store');
  const listeners = new Map<string, (e: KeyboardEvent) => void>();
  const doc = {
    addEventListener: (type: string, listener: (e: KeyboardEvent) => void) => {
      listeners.set(type, listener);
    },
    removeEventListener: (type: string, listener: (e: KeyboardEvent) => void) => {
      if (listeners.get(type) === listener) {
        listeners.delete(type);
      }
    },
  };
  const ran: ActionName[] = [];
  const remove = installKeys(doc as unknown as Document, (a) => ran.push(a));
  const send = (pressed: Pressed): Pressed => {
    listeners.get('keydown')?.(pressed.event);
    return pressed;
  };
  const filterBox: Target = { tagName: 'INPUT', hasAttribute: (name) => name === filterBoxAttribute };
  return { useApp, ran, remove, send, listeners, filterBox };
}

beforeEach(() => {
  vi.resetModules();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('installKeys', () => {
  it('runs the action of the pane that has focus', async () => {
    const { useApp, ran, send } = await setup();

    const first = send(press('p'));
    useApp.setState({ focus: 'inspector' });
    send(press('p'));

    expect(ran).toEqual(['addToPlaylist', 'hear']);
    expect(first.prevented()).toBe(true);
  });

  it('a key typed in an INPUT, TEXTAREA or SELECT is ignored', async () => {
    const { ran, send } = await setup();

    for (const tagName of ['INPUT', 'TEXTAREA', 'SELECT']) {
      const typed = send(press('j', {}, { tagName }));
      expect(typed.prevented(), tagName).toBe(false);
      send(press('r', { ctrl: true }, { tagName }));
    }

    expect(ran).toEqual([]);
  });

  it('isComposing ignores the key', async () => {
    const { ran, send } = await setup();

    const composed = send(press('j', { composing: true }));

    expect(ran).toEqual([]);
    expect(composed.prevented()).toBe(false);
  });

  it('meta and alt combinations are ignored', async () => {
    const { ran, send } = await setup();

    const pressed = [
      send(press('j', { meta: true })),
      send(press('j', { alt: true })),
      send(press('r', { ctrl: true, meta: true })),
      send(press('a', { ctrl: true, alt: true })),
    ];

    expect(ran).toEqual([]);
    expect(pressed.map((p) => p.prevented())).toEqual([false, false, false, false]);
  });

  it('ctrl+r, ctrl+a, ctrl+u, ctrl+d resolve when focus is not in an input', async () => {
    const { ran, send } = await setup();

    const pressed = ['r', 'a', 'u', 'd'].map((key) => send(press(key, { ctrl: true })));

    expect(ran).toEqual(['redo', 'markAll', 'pageUp', 'pageDown']);
    expect(pressed.map((p) => p.prevented())).toEqual([true, true, true, true]);
  });

  it('the filter box handles only Enter, ArrowDown and Escape', async () => {
    const { ran, send, filterBox } = await setup();

    const handled = ['Enter', 'ArrowDown', 'Escape'].map((key) => send(press(key, {}, filterBox)));
    const left = [send(press('j', {}, filterBox)), send(press('?', {}, filterBox)), send(press('ArrowUp', {}, filterBox)), send(press('a', { ctrl: true }, filterBox))];

    expect(ran).toEqual(['filterAccept', 'filterAccept', 'filterClear']);
    expect(handled.map((p) => p.prevented())).toEqual([true, true, true]);
    expect(left.map((p) => p.prevented())).toEqual([false, false, false, false]);
  });

  it('the filter box ignores a key being composed, and meta or alt combinations', async () => {
    const { ran, send, filterBox } = await setup();

    const pressed = [
      send(press('Enter', { composing: true }, filterBox)),
      send(press('Enter', { meta: true }, filterBox)),
      send(press('Enter', { alt: true }, filterBox)),
      send(press('Escape', { composing: true }, filterBox)),
    ];

    expect(ran).toEqual([]);
    expect(pressed.map((p) => p.prevented())).toEqual([false, false, false, false]);
  });

  it('an input that is not the filter box does not take the filter keys', async () => {
    const { ran, send } = await setup();

    send(press('Escape', {}, { tagName: 'INPUT', hasAttribute: () => false }));

    expect(ran).toEqual([]);
  });

  it('keys wait while busy', async () => {
    const { useApp, ran, send } = await setup();

    useApp.setState({ busy: true });
    const waiting = send(press('j'));
    send(press('u'));
    useApp.setState({ busy: false });
    send(press('k'));

    expect(ran).toEqual(['moveUp']);
    expect(waiting.prevented()).toBe(true);
  });

  it('the filter box keys do not wait while busy, as on the old page', async () => {
    const { useApp, ran, send, filterBox } = await setup();

    useApp.setState({ busy: true });
    send(press('Escape', {}, filterBox));

    expect(ran).toEqual(['filterClear']);
  });

  it('while a dialog is open the panes leave the keyboard to it', async () => {
    const { useApp, ran, send, filterBox } = await setup();

    useApp.setState({ overlay: { kind: 'help' } });
    const pressed = [send(press('?')), send(press('Escape')), send(press('r', { ctrl: true })), send(press('Enter', {}, filterBox))];

    expect(ran).toEqual([]);
    expect(pressed.map((p) => p.prevented())).toEqual([false, false, false, false]);
  });

  it('only a key that resolves is prevented', async () => {
    const { ran, send } = await setup();

    const pressed = [send(press('c', { ctrl: true })), send(press('q')), send(press('F5'))];

    expect(ran).toEqual([]);
    expect(pressed.map((p) => p.prevented())).toEqual([false, false, false]);
  });

  it('the remover takes the listener away', async () => {
    const { remove, listeners } = await setup();

    remove();

    expect(listeners.has('keydown')).toBe(false);
  });
});
