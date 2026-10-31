import { useId, useLayoutEffect, useRef, type KeyboardEvent, type MouseEvent, type ReactNode } from 'react';
import type { KeyEventLike } from '../keys/keymap';
import { useDialogHints, type Hint } from './hints';
import { closeOverlay } from './open';

export interface DialogProps {
  title: ReactNode;
  hints: readonly Hint[];
  onKey: (e: KeyEventLike) => boolean;
  children: ReactNode;
}

const tabbable = 'button, input, select, textarea, a[href], [tabindex]:not([tabindex="-1"])';

function nextTabStop(dialog: HTMLElement, back: boolean): HTMLElement {
  const stops = [...dialog.querySelectorAll<HTMLElement>(tabbable)].filter((el) => !el.matches(':disabled'));
  const at = stops.findIndex((el) => el === document.activeElement);
  const next = back ? (at <= 0 ? stops.length - 1 : at - 1) : (at + 1) % stops.length;
  return stops[next] ?? dialog;
}

export function Dialog({ title, hints, onKey, children }: DialogProps) {
  const titleId = useId();
  const ref = useRef<HTMLDivElement>(null);

  useLayoutEffect(() => {
    const dialog = ref.current;
    const before = document.activeElement;
    dialog?.focus({ preventScroll: true });
    const keepFocus = (e: FocusEvent) => {
      if (dialog !== null && e.target instanceof Node && !dialog.contains(e.target)) {
        dialog.focus({ preventScroll: true });
      }
    };
    document.addEventListener('focusin', keepFocus);
    return () => {
      document.removeEventListener('focusin', keepFocus);
      useDialogHints.setState({ hints: [] });
      if (before instanceof HTMLElement && before.isConnected) {
        before.focus({ preventScroll: true });
      }
    };
  }, []);

  useLayoutEffect(() => {
    useDialogHints.setState({ hints });
  }, [hints]);

  useLayoutEffect(() => {
    const dialog = ref.current;
    if (dialog !== null && !dialog.contains(document.activeElement)) {
      dialog.focus({ preventScroll: true });
    }
  });

  function keyDown(e: KeyboardEvent<HTMLDivElement>): void {
    e.stopPropagation();
    const native = e.nativeEvent;
    if (native.isComposing || e.ctrlKey || e.metaKey || e.altKey) {
      return;
    }
    if (onKey(native)) {
      e.preventDefault();
    } else if (e.key === 'Escape') {
      e.preventDefault();
      closeOverlay();
    } else if (e.key === 'Tab' && ref.current !== null) {
      e.preventDefault();
      nextTabStop(ref.current, e.shiftKey).focus();
    }
  }

  function mouseDown(e: MouseEvent<HTMLDivElement>): void {
    if (e.target === e.currentTarget) {
      closeOverlay();
    }
  }

  return (
    <div className="backdrop" data-testid="backdrop" onMouseDown={mouseDown}>
      <div ref={ref} className="dialog" role="dialog" aria-modal="true" aria-labelledby={titleId} tabIndex={-1} onKeyDown={keyDown}>
        <h2 id={titleId} className="dialog-title">
          {title}
        </h2>
        <div className="dialog-body">{children}</div>
      </div>
    </div>
  );
}
