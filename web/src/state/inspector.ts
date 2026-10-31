import type { FieldView, PeekView, TrackView } from '../api/types.gen';
import { useApp, type AppState } from './store';

type Shown = Pick<AppState, 'detail' | 'tab' | 'field'>;

export const nudgeable: readonly string[] = ['cutStart', 'cutEnd', 'volume', 'fadeIn', 'fadeOut'];

export function isTrack(detail: TrackView | PeekView | null): detail is TrackView {
  return detail !== null && 'tabs' in detail;
}

export function tabIndex(view: TrackView, tab: number): number {
  return Math.max(Math.min(tab, view.tabs.length - 1), 0);
}

export function fieldList(s: Pick<AppState, 'detail' | 'tab'>): FieldView[] {
  const d = s.detail;
  if (d === null) {
    return [];
  }
  if (isTrack(d)) {
    return d.tabs[tabIndex(d, s.tab)]?.fields ?? [];
  }
  return d.fields ?? [];
}

export function fieldIndex(s: Shown): number {
  return Math.max(Math.min(s.field, fieldList(s).length - 1), 0);
}

export function currentField(s: Shown & Pick<AppState, 'focus'>): FieldView | undefined {
  if (s.focus !== 'inspector') {
    return undefined;
  }
  return fieldList(s)[fieldIndex(s)];
}

export function hasSecondNumber(s: Shown & Pick<AppState, 'focus' | 'edit'>): boolean {
  return s.edit !== null && isTrack(s.detail) && currentField(s)?.key === 'track';
}

export function moveField(n: number): void {
  const s = useApp.getState();
  const count = fieldList(s).length;
  if (count === 0) {
    return;
  }
  useApp.setState({ field: Math.min(Math.max(fieldIndex(s) + n, 0), count - 1) });
}

export function chooseTab(i: number): void {
  const d = useApp.getState().detail;
  if (!isTrack(d) || i < 0 || i >= d.tabs.length) {
    return;
  }
  useApp.setState({ tab: i, field: 0, edit: null });
}

export function nextTab(): void {
  const { detail, tab } = useApp.getState();
  if (isTrack(detail)) {
    chooseTab((tabIndex(detail, tab) + 1) % detail.tabs.length);
  }
}

export function previousTabOrBack(): void {
  const { detail, tab } = useApp.getState();
  if (isTrack(detail) && tabIndex(detail, tab) > 0) {
    chooseTab(tabIndex(detail, tab) - 1);
    return;
  }
  useApp.setState({ focus: 'tree' });
}

