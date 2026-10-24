import { get, del } from './api.js';

// A job is work the server does on its own while the page shows how far it
// has got. The server tells every page about each step as a "job" event; a
// page also asks now and then, in case an event was missed.

const watchers = new Map();

// jobEvent is what the stream says about a job.
export function jobEvent(view) {
  const watch = watchers.get(view.id);
  if (watch) watch(view);
}

// follow calls update with the job as it is each time it changes, and resolves
// with the job once it is no longer running.
export function follow(view, update) {
  return new Promise((resolve) => {
    let finished = false;
    let timer = 0;
    const step = (v) => {
      if (finished) return;
      update(v);
      if (v.state !== 'running') {
        finished = true;
        clearInterval(timer);
        watchers.delete(v.id);
        resolve(v);
      }
    };
    watchers.set(view.id, step);
    timer = setInterval(async () => {
      try { step(await get(`/api/jobs/${encodeURIComponent(view.id)}`)); } catch { /* the next time */ }
    }, 1500);
    step(view);
  });
}

export const stopJob = (id) => del(`/api/jobs/${encodeURIComponent(id)}`).catch(() => {});
