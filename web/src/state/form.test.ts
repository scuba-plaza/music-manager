import { describe, expect, it } from 'vitest';
import { cleanChecks, firstWrong, shownErrors, tried, verdict, left } from './form';

const wrong = { title: 'Title cannot be empty', released: 'Not a date' };

describe('what a form shows of what the server found wrong', () => {
  it('shows nothing before the person has left a field or tried to save', () => {
    const checks = verdict(cleanChecks(), wrong);

    expect(shownErrors(checks)).toEqual({});
  });

  it('shows the message of a field once it has been left, and only that one', () => {
    const checks = left(verdict(cleanChecks(), wrong), 'released');

    expect(shownErrors(checks)).toEqual({ released: 'Not a date' });
  });

  it('shows every message after a try to save', () => {
    const checks = tried(verdict(cleanChecks(), wrong));

    expect(shownErrors(checks)).toEqual(wrong);
  });

  it('takes a message away as soon as the server no longer has it', () => {
    const before = left(verdict(cleanChecks(), wrong), 'released');

    const after = verdict(before, { title: 'Title cannot be empty' });

    expect(shownErrors(after)).toEqual({});
  });

  it('keeps a field as left when a new verdict comes', () => {
    const checks = verdict(left(cleanChecks(), 'released'), wrong);

    expect(shownErrors(checks)).toEqual({ released: 'Not a date' });
  });
});

describe('firstWrong', () => {
  const order = ['title', 'artist', 'released'];

  it('is the first field in the order of the form that shows a message', () => {
    const checks = tried(verdict(cleanChecks(), { released: 'Not a date', artist: 'Artist cannot be empty' }));

    expect(firstWrong(checks, order)).toBe('artist');
  });

  it('is nothing when no message is shown', () => {
    expect(firstWrong(verdict(cleanChecks(), wrong), order)).toBeUndefined();
  });

  it('skips a message about a field that is not in the form', () => {
    const checks = tried(verdict(cleanChecks(), { elsewhere: 'x' }));

    expect(firstWrong(checks, order)).toBeUndefined();
  });
});
