import { filterBoxAttribute } from '../keys/dispatch';

function treeElement(): HTMLElement | null {
  return document.querySelector<HTMLElement>('[role="tree"]');
}

function cursorElement(): HTMLElement | null {
  return treeElement()?.querySelector<HTMLElement>('[aria-selected="true"]') ?? null;
}

function filterBox(): HTMLInputElement | null {
  return document.querySelector<HTMLInputElement>(`[${filterBoxAttribute}]`);
}

export function pageSize(): number {
  const tree = treeElement();
  const rowHeight = tree?.querySelector<HTMLElement>('[role="treeitem"]')?.offsetHeight ?? 0;
  if (tree === null || rowHeight === 0) {
    return 1;
  }
  return Math.max(1, Math.floor(tree.clientHeight / rowHeight) - 1);
}

export function centreCursor(): void {
  cursorElement()?.scrollIntoView({ block: 'center' });
}

export function scrollCursorIntoView(): void {
  cursorElement()?.scrollIntoView({ block: 'nearest' });
}

export function focusFilterBox(): void {
  const box = filterBox();
  box?.focus();
  box?.select();
}

export function leaveFilterBox(): void {
  filterBox()?.blur();
  treeElement()?.focus({ preventScroll: true });
}
