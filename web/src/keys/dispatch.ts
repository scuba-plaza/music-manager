import { useApp } from '../state/store';
import { resolve, resolveFilter, type ActionName } from './keymap';

export const filterBoxAttribute = 'data-filter-box';

const inputTags = new Set(['INPUT', 'TEXTAREA', 'SELECT']);

export function installKeys(target: Document, run: (a: ActionName) => void): () => void {
  const onKeydown = (e: KeyboardEvent) => {
    const { overlay, focus, busy } = useApp.getState();
    if (overlay !== null) {
      return;
    }
    const el = e.target as Partial<Element> | null;
    if (el?.hasAttribute?.(filterBoxAttribute)) {
      const action = resolveFilter(e);
      if (action !== undefined) {
        e.preventDefault();
        run(action);
      }
      return;
    }
    if (inputTags.has(el?.tagName ?? '')) {
      return;
    }
    const action = resolve(focus, e);
    if (action === undefined) {
      return;
    }
    e.preventDefault();
    if (busy) {
      return;
    }
    run(action);
  };
  target.addEventListener('keydown', onKeydown);
  return () => {
    target.removeEventListener('keydown', onKeydown);
  };
}
