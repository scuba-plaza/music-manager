import { get, send } from '../api/client';
import type { JobView } from '../api/types.gen';

const askEvery = 1500;
const watchers = new Map<string, (view: JobView) => void>();

export function jobEvent(view: JobView): void {
  watchers.get(view.id)?.(view);
}

export function follow(view: JobView, update: (view: JobView) => void): Promise<JobView> {
  return new Promise((resolve) => {
    let finished = false;
    const timer = setInterval(() => {
      get<JobView>(`/api/jobs/${encodeURIComponent(view.id)}`).then(step, () => undefined);
    }, askEvery);
    function step(v: JobView): void {
      if (finished) {
        return;
      }
      update(v);
      if (v.state !== 'running') {
        finished = true;
        clearInterval(timer);
        watchers.delete(v.id);
        resolve(v);
      }
    }
    watchers.set(view.id, step);
    step(view);
  });
}

export async function stopJob(id: string): Promise<void> {
  try {
    await send<JobView>('DELETE', `/api/jobs/${encodeURIComponent(id)}`);
  } catch {
    return;
  }
}
