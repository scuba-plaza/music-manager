import { get } from '../api/client';
import type { ProfileView } from '../api/types.gen';
import { fail, loadDetail } from './actions';
import { isTrack } from './inspector';
import { useApp } from './store';

const generic = 'generic';

export async function loadProfiles(): Promise<void> {
  try {
    const profiles = await get<ProfileView[]>('/api/profiles');
    const { profile, detail } = useApp.getState();
    const chosen = profiles.some((p) => p.name === profile) ? profile : (profiles.find((p) => p.name !== generic)?.name ?? '');
    useApp.setState({ profiles, profile: chosen });
    if (chosen !== profile && isTrack(detail)) {
      await loadDetail();
    }
  } catch (err) {
    fail(err);
  }
}

export async function chooseProfile(name: string): Promise<void> {
  useApp.setState({ profile: name });
  await loadDetail();
}
