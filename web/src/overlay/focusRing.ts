import type { KeyEventLike } from '../keys/keymap';

function step(e: KeyEventLike, arrows: boolean): number | undefined {
  if (e.key === 'Tab') {
    return e.shiftKey ? -1 : 1;
  }
  if (arrows && e.key === 'ArrowDown') {
    return 1;
  }
  return arrows && e.key === 'ArrowUp' ? -1 : undefined;
}

export function ringKey(e: KeyEventLike, controls: readonly HTMLElement[], arrows = true): boolean {
  const by = step(e, arrows);
  if (by === undefined) {
    return false;
  }
  const at = Math.max(
    0,
    controls.findIndex((c) => c === document.activeElement),
  );
  controls[(at + by + controls.length) % controls.length]?.focus();
  return true;
}
