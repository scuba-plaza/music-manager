import type { KeyEventLike } from '../keys/keymap';
import { Dialog } from '../overlay/Dialog';
import { Progress } from '../components/Progress';
import { progressOf, stopOrClose, useIdentifyAll } from '../state/identifyAll';

function onKey(e: KeyEventLike): boolean {
  if (e.key !== 'Escape' && e.key !== 'Enter') {
    return false;
  }
  stopOrClose();
  return true;
}

export function IdentifyAll() {
  const job = useIdentifyAll((s) => s.job);
  const progress = progressOf(job);
  const running = job?.state === 'running';
  let head = 'Done';
  if (running) {
    head = 'Identifying tracks on MusicBrainz…';
  } else if (job?.state === 'stopped') {
    head = 'Stopped';
  } else if (job?.state === 'failed') {
    head = job.error ?? 'Failed';
  }
  const failures = progress.failures.slice(0, 3);

  return (
    <Dialog title="Identify all" hints={running ? [{ keys: 'esc', label: 'stop' }] : [{ keys: 'enter', label: 'close' }]} onKey={onKey}>
      <div className="ask-head">
        <bdi dir="auto">{head}</bdi>
      </div>
      <Progress done={progress.done} total={progress.total} label="Identified so far" />
      <div>{`Identified ${String(progress.identified)} · Left for you ${String(progress.left)} · Failed ${String(progress.failed)}`}</div>
      {failures.map((f) => (
        <div className="warn" key={f}>
          <bdi dir="auto">{f}</bdi>
        </div>
      ))}
      {progress.failures.length > 3 && <div className="dim">…and more</div>}
      {!running && progress.left > 0 && <div className="ask-body">The rest need a closer look: i searches for one track, o uses its old tags.</div>}
    </Dialog>
  );
}
