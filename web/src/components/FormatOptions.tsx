import type { Profile } from '../api/types.gen';
import { formatChoices } from '../state/profiles';
import { useApp } from '../state/store';

const noFormats: Profile[] = [];

export function useStoredFormats(): readonly Profile[] {
  return useApp((s) => s.lib?.profiles ?? noFormats);
}

export function FormatOptions() {
  return formatChoices(useStoredFormats()).map((p) => (
    <option key={p.id} value={p.id}>
      {p.name}
    </option>
  ));
}
