import { create } from 'zustand';
import { send } from '../api/client';
import type { IdentifyProgress, JobView } from '../api/types.gen';
import { loadLibrary } from './library';
import { fail, setStatus } from './status';
import { needLookup } from './identify';
import { follow, stopJob } from './jobs';
import { useApp } from './store';
import { plural } from './text';

export const useIdentifyAll = create<{ job: JobView | null }>()(() => ({ job: null }));

const nothingDone: IdentifyProgress = { total: 0, done: 0, identified: 0, left: 0, failed: 0, failures: [] };

function isProgress(data: unknown): data is IdentifyProgress {
  return (
    typeof data === 'object' &&
    data !== null &&
    'total' in data &&
    typeof data.total === 'number' &&
    'done' in data &&
    typeof data.done === 'number' &&
    'identified' in data &&
    typeof data.identified === 'number' &&
    'left' in data &&
    typeof data.left === 'number' &&
    'failed' in data &&
    typeof data.failed === 'number' &&
    'failures' in data &&
    Array.isArray(data.failures)
  );
}

export function progressOf(job: JobView | null): IdentifyProgress {
  return job !== null && isProgress(job.data) ? job.data : nothingDone;
}

async function start(): Promise<void> {
  let view: JobView;
  try {
    view = await send<JobView>('POST', '/api/identify-all');
  } catch (err) {
    fail(err);
    return;
  }
  useIdentifyAll.setState({ job: view });
  useApp.setState({ overlay: { kind: 'identifyAll' } });
  await follow(view, (v) => {
    useIdentifyAll.setState({ job: v });
  });
  await loadLibrary();
}

export function identifyAll(): void {
  if (!needLookup()) {
    return;
  }
  const need = useApp.getState().lib?.view.needWork ?? 0;
  if (need === 0) {
    setStatus('Nothing needs work');
    return;
  }
  useApp.setState({
    overlay: {
      kind: 'ask',
      head: `Identify ${plural(need, 'track')} on MusicBrainz?`,
      body: [
        'Each one is searched for, and the best match is taken only if it is a confident one (a score of 95 or more). The rest are left alone for you to look at.',
        'This takes about a second per track, and u undoes it all in one step.',
      ],
      yes: 'identify',
      onYes: () => {
        void start();
      },
    },
  });
}

export function stopOrClose(): void {
  const { job } = useIdentifyAll.getState();
  if (job?.state === 'running') {
    void stopJob(job.id);
    return;
  }
  useApp.setState({ overlay: null });
}
