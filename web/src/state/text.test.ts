import { describe, expect, it } from 'vitest';
import { plural } from './text';

describe('plural', () => {
  it('says one of something without an s', () => {
    expect(plural(1, 'track')).toBe('1 track');
  });

  it('adds an s for none and for several', () => {
    expect(plural(0, 'track')).toBe('0 tracks');
    expect(plural(2, 'warning')).toBe('2 warnings');
  });

  it('adds es after ch, sh, s and x', () => {
    expect(plural(3, 'match')).toBe('3 matches');
    expect(plural(2, 'wish')).toBe('2 wishes');
    expect(plural(2, 'bus')).toBe('2 buses');
    expect(plural(2, 'box')).toBe('2 boxes');
    expect(plural(1, 'match')).toBe('1 match');
  });
});
