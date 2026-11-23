import { describe, expect, it } from 'vitest';
import { plural } from './text';

describe('plural', () => {
  it.each([
    [1, 'track', '1 track'],
    [0, 'track', '0 tracks'],
    [2, 'warning', '2 warnings'],
    [3, 'match', '3 matches'],
    [2, 'wish', '2 wishes'],
    [2, 'bus', '2 buses'],
    [2, 'box', '2 boxes'],
    [1, 'match', '1 match'],
  ])('%i %s is %s', (n, word, expected) => {
    expect(plural(n, word)).toBe(expected);
  });
});
