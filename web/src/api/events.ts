import type { JobView } from './types.gen';

export interface Handlers {
  library(stamp: string): void;
  job(e: JobView): void;
  open(): void;
}

function payload(e: MessageEvent): unknown {
  const raw: unknown = e.data;
  if (typeof raw !== 'string') {
    return undefined;
  }
  try {
    return JSON.parse(raw);
  } catch {
    return undefined;
  }
}

function isJobView(value: unknown): value is JobView {
  return (
    typeof value === 'object' &&
    value !== null &&
    'id' in value &&
    typeof value.id === 'string' &&
    'kind' in value &&
    typeof value.kind === 'string' &&
    'state' in value &&
    typeof value.state === 'string'
  );
}

export function subscribe(h: Handlers): () => void {
  const source = new EventSource('/api/events');
  let seen: string | null = null;
  source.addEventListener('library', (e) => {
    const stamp = payload(e);
    if (typeof stamp !== 'string') {
      h.library('');
      return;
    }
    if (stamp === seen) {
      return;
    }
    seen = stamp;
    h.library(stamp);
  });
  source.addEventListener('job', (e) => {
    const job = payload(e);
    if (isJobView(job)) {
      h.job(job);
    }
  });
  source.addEventListener('open', () => {
    h.open();
  });
  return () => {
    source.close();
  };
}
