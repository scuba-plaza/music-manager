import { describe, expect, it } from 'vitest';
import { answerOf, closes, motionOf, moveTo } from './dialog';
import type { KeyEventLike } from './keymap';

function press(key: string, mods: Partial<KeyEventLike> = {}): KeyEventLike {
  return { key, ctrlKey: false, metaKey: false, altKey: false, shiftKey: false, isComposing: false, ...mods };
}

describe('motionOf', () => {
  it('moves by a line with the arrows and with j and k', () => {
    expect(motionOf(press('ArrowUp'))).toBe('up');
    expect(motionOf(press('k'))).toBe('up');
    expect(motionOf(press('ArrowDown'))).toBe('down');
    expect(motionOf(press('j'))).toBe('down');
  });

  it('goes to the ends with Home and g, End and G', () => {
    expect(motionOf(press('Home'))).toBe('first');
    expect(motionOf(press('g'))).toBe('first');
    expect(motionOf(press('End'))).toBe('last');
    expect(motionOf(press('G', { shiftKey: true }))).toBe('last');
  });

  it('is not changed by Caps Lock: a capital G without Shift still goes to the top', () => {
    expect(motionOf(press('G'))).toBe('first');
    expect(motionOf(press('g', { shiftKey: true }))).toBe('last');
  });

  it('has no page keys, and nothing else moves', () => {
    for (const key of ['PageUp', 'PageDown', 'q', 'Enter', ' ', 'h', 'l', 'Tab']) {
      expect(motionOf(press(key)), key).toBeUndefined();
    }
  });
});

describe('closes', () => {
  it('is Escape or q', () => {
    expect(closes(press('Escape'))).toBe(true);
    expect(closes(press('q'))).toBe(true);
    expect(closes(press('Q'))).toBe(true);
  });

  it('is nothing else, and not q with Shift held', () => {
    for (const key of ['Enter', 'n', 'x', '?']) {
      expect(closes(press(key)), key).toBe(false);
    }
    expect(closes(press('Q', { shiftKey: true }))).toBe(false);
  });
});

describe('answerOf', () => {
  it('is y for yes and n for no, and nothing else answers', () => {
    expect(answerOf(press('y'))).toBe('yes');
    expect(answerOf(press('n'))).toBe('no');
    for (const key of ['Enter', 'Escape', 'q', ' ', 'Y', 'N']) {
      const shifted = key === 'Y' || key === 'N';
      expect(answerOf(press(key, { shiftKey: shifted })), key).toBeUndefined();
    }
  });
});

describe('moveTo', () => {
  it('moves a line and stops at the ends', () => {
    expect(moveTo(5, 2, 'up')).toBe(1);
    expect(moveTo(5, 2, 'down')).toBe(3);
    expect(moveTo(5, 0, 'up')).toBe(0);
    expect(moveTo(5, 4, 'down')).toBe(4);
  });

  it('goes to the first and the last', () => {
    expect(moveTo(5, 2, 'first')).toBe(0);
    expect(moveTo(5, 2, 'last')).toBe(4);
  });

  it('brings a cursor that is past the end back into the list', () => {
    expect(moveTo(3, 9, 'up')).toBe(2);
    expect(moveTo(3, 9, 'down')).toBe(2);
  });

  it('stays at the start of an empty list', () => {
    expect(moveTo(0, 0, 'down')).toBe(0);
    expect(moveTo(0, 0, 'last')).toBe(0);
  });
});
