import { useLayoutEffect, useRef } from 'react';
import type { KeyEventLike } from '../keys/keymap';
import { Dialog } from '../overlay/Dialog';
import { closeOverlay } from '../state/store';
import { chooseProblem, goToProblem, moveProblem, useDoctor } from '../state/doctor';
import { plural } from '../state/text';

function onKey(e: KeyEventLike): boolean {
  if (e.key === 'q') {
    closeOverlay();
    return true;
  }
  if (e.key === 'ArrowUp' || e.key === 'k') {
    moveProblem(-1);
  } else if (e.key === 'ArrowDown' || e.key === 'j') {
    moveProblem(1);
  } else if (e.key === 'Enter') {
    goToProblem();
  } else {
    return false;
  }
  return true;
}

export function Doctor() {
  const { phase, result, error, cursor } = useDoctor();
  const list = useRef<HTMLDivElement>(null);

  useLayoutEffect(() => {
    list.current?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: 'nearest' });
  }, [cursor, phase]);

  const hints =
    phase === 'done' && result !== null && result.problems.length > 0
      ? [
          { keys: '↑↓', label: 'choose' },
          { keys: 'enter', label: 'go to the track' },
          { keys: 'esc', label: 'close' },
        ]
      : [{ keys: 'esc', label: 'close' }];

  return (
    <Dialog title="Library check" hints={hints} onKey={onKey}>
      {phase === 'checking' && (
        <>
          <div className="busy">Checking that every stored file is there and unharmed…</div>
          <div className="ask-body">esc cancels</div>
        </>
      )}
      {phase === 'failed' && (
        <div className="bad" role="alert">
          {error}
        </div>
      )}
      {phase === 'done' && result !== null && result.problems.length === 0 && <div className="good">No problems found.</div>}
      {phase === 'done' && result !== null && result.problems.length > 0 && (
        <>
          <div className="ask-head">{`${plural(result.errors, 'error')}, ${plural(result.warnings, 'warning')}`}</div>
          <div ref={list} className="problems" role="listbox" aria-label="Problems">
            {result.problems.map((p, i) => (
              <div
                key={`${p.trackId}-${String(i)}`}
                role="option"
                aria-selected={i === cursor}
                className={i === cursor ? 'problem on' : 'problem'}
                onClick={() => {
                  chooseProblem(i);
                  goToProblem();
                }}
              >
                <span className={p.severity === 'error' ? 'bad' : 'warn'} aria-label={p.severity}>
                  {p.severity === 'error' ? '✗' : '!'}
                </span>
                <b>
                  <bdi dir="auto">{p.title}</bdi>
                </b>
                <span className="dim">
                  {' · '}
                  <bdi dir="auto">{p.message}</bdi>
                </span>
              </div>
            ))}
          </div>
        </>
      )}
    </Dialog>
  );
}
