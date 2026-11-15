import type { Profile } from '../api/types.gen';
import { loadDetail } from './detail';
import { useApp } from './store';

export const genericId = 'generic';

export interface FormatChoice {
  id: string;
  name: string;
}

export function previewChoice(stored: readonly Profile[], current: string): string {
  if (stored.length === 0) {
    return '';
  }
  return current === genericId || stored.some((p) => p.id === current) ? current : (stored[0]?.id ?? '');
}

export function formatChoices(stored: readonly Profile[]): FormatChoice[] {
  return [{ id: genericId, name: 'Generic' }, ...stored.map((p) => ({ id: p.id, name: p.name }))];
}

export async function chooseProfile(id: string): Promise<void> {
  useApp.setState({ profile: id });
  await loadDetail();
}
