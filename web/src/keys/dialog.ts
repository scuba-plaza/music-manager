import { keyName, type KeyEventLike } from './keymap';

export type Motion = 'up' | 'down' | 'first' | 'last';

export type Answer = 'yes' | 'no';

const motions = new Map<string, Motion>([
  ['ArrowUp', 'up'],
  ['k', 'up'],
  ['ArrowDown', 'down'],
  ['j', 'down'],
  ['Home', 'first'],
  ['g', 'first'],
  ['End', 'last'],
  ['Shift+g', 'last'],
]);

const answers = new Map<string, Answer>([
  ['y', 'yes'],
  ['n', 'no'],
]);

export function motionOf(e: KeyEventLike): Motion | undefined {
  return motions.get(keyName(e));
}

export function closes(e: KeyEventLike): boolean {
  const name = keyName(e);
  return name === 'Escape' || name === 'q';
}

export function answerOf(e: KeyEventLike): Answer | undefined {
  return answers.get(keyName(e));
}

export function moveTo(length: number, at: number, motion: Motion): number {
  const last = Math.max(length - 1, 0);
  const clamp = (n: number) => Math.min(Math.max(n, 0), last);
  switch (motion) {
    case 'up':
      return clamp(at - 1);
    case 'down':
      return clamp(at + 1);
    case 'first':
      return 0;
    case 'last':
      return last;
  }
}
