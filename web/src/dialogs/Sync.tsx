import { useEffect, useRef } from 'react';
import { Progress } from '../components/Progress';
import type { KeyEventLike } from '../keys/keymap';
import { Dialog } from '../overlay/Dialog';
import { MenuList } from '../overlay/MenuList';
import { closeOverlay } from '../overlay/open';
import { useApp } from '../state/store';
import {
  applyPlan,
  backToChoosing,
  backToReview,
  chooseProfile,
  chooseTarget,
  confirmDelete,
  cycleProfile,
  moveTarget,
  pickTarget,
  progressOf,
  submitFolder,
  togglePrune,
  typeConfirm,
  typeFolder,
  useSync,
  type SyncPhase,
} from '../state/sync';
import { plural } from '../state/text';

const glyphs: Record<string, string> = { create: '+', update: '~', remove: '−', stray: '!' };
const warningsShown = 4;

interface Hint {
  keys: string;
  label: string;
}

function hintsFor(phase: SyncPhase, library: boolean, nothingToDo: boolean): readonly Hint[] {
  switch (phase) {
    case 'choosing':
      return [
        { keys: '↑↓', label: 'choose' },
        { keys: 'enter', label: 'choose' },
        ...(library ? [] : [{ keys: 'w', label: 'format' }]),
        { keys: 'esc', label: 'close' },
      ];
    case 'typing':
      return [
        { keys: 'enter', label: 'plan' },
        { keys: 'esc', label: 'back' },
      ];
    case 'planning':
      return [{ keys: 'esc', label: 'cancel' }];
    case 'reviewing':
      return nothingToDo
        ? [{ keys: 'enter', label: 'close' }]
        : [
            { keys: '↑↓', label: 'scroll' },
            { keys: 'enter', label: 'apply' },
            { keys: 'p', label: 'strays' },
            { keys: 'esc', label: 'close' },
          ];
    case 'confirming':
      return [
        { keys: 'enter', label: 'confirm' },
        { keys: 'esc', label: 'back' },
      ];
    case 'applying':
      return [];
    case 'finished':
    case 'failed':
      return [{ keys: 'enter', label: 'close' }];
  }
}

function Format({ library, how }: { library: boolean; how: string }) {
  const profiles = useApp((s) => s.profiles);
  const profile = useSync((s) => s.profile);
  if (library) {
    return (
      <div className="format">
        <span className="dim">Format </span>
        Generic <span className="dim">· the library folder is always written the standard way</span>
      </div>
    );
  }
  return (
    <label className="format">
      <span className="dim">Format </span>
      <select
        className="edit"
        aria-label="Format"
        value={profile}
        onChange={(e) => {
          chooseProfile(e.target.value);
        }}
      >
        {profiles.map((p) => (
          <option key={p.name} value={p.name}>
            {p.label}
          </option>
        ))}
      </select>
      {how !== '' && <span className="dim"> · {how}</span>}
    </label>
  );
}

export function Sync() {
  const { phase, targets, cursor, folder, error, where, job, plan, summary, confirm } = useSync();
  const folderInput = useRef<HTMLInputElement>(null);
  const confirmInput = useRef<HTMLInputElement>(null);
  const ops = useRef<HTMLDivElement>(null);
  const library = targets[cursor]?.kind === 'library';

  useEffect(() => {
    if (phase === 'typing') {
      folderInput.current?.focus();
    } else if (phase === 'confirming') {
      confirmInput.current?.focus();
    }
  }, [phase]);

  function scroll(lines: number): void {
    const list = ops.current;
    if (list === null) {
      return;
    }
    const row = list.firstElementChild?.getBoundingClientRect().height ?? 20;
    if (lines === Infinity) {
      list.scrollTop = list.scrollHeight;
    } else if (lines === -Infinity) {
      list.scrollTop = 0;
    } else {
      list.scrollTop += lines * row;
    }
  }

  function onKey(e: KeyEventLike): boolean {
    const inSelect = document.activeElement instanceof HTMLSelectElement;
    switch (phase) {
      case 'choosing':
        if (inSelect) {
          return false;
        }
        if (e.key === 'q') {
          closeOverlay();
        } else if (e.key === 'ArrowUp' || e.key === 'k') {
          moveTarget(-1);
        } else if (e.key === 'ArrowDown' || e.key === 'j') {
          moveTarget(1);
        } else if (e.key === 'Home') {
          chooseTarget(0);
        } else if (e.key === 'End') {
          chooseTarget(targets.length - 1);
        } else if (e.key === 'w') {
          cycleProfile();
        } else if (e.key === 'Enter') {
          pickTarget();
        } else {
          return false;
        }
        return true;
      case 'typing':
        if (e.key === 'Escape') {
          backToChoosing();
        } else if (e.key === 'Enter' && !inSelect) {
          submitFolder();
        } else {
          return false;
        }
        return true;
      case 'planning':
      case 'applying':
        return false;
      case 'reviewing':
        if (e.key === 'q') {
          closeOverlay();
        } else if (e.key === 'ArrowUp' || e.key === 'k') {
          scroll(-1);
        } else if (e.key === 'ArrowDown' || e.key === 'j') {
          scroll(1);
        } else if (e.key === 'PageUp') {
          scroll(-10);
        } else if (e.key === 'PageDown') {
          scroll(10);
        } else if (e.key === 'Home') {
          scroll(-Infinity);
        } else if (e.key === 'End') {
          scroll(Infinity);
        } else if (e.key === 'p' && plan !== null && plan.ops.length > 0) {
          void togglePrune();
        } else if (e.key === 'Enter') {
          applyPlan();
        } else {
          return false;
        }
        return true;
      case 'confirming':
        if (e.key === 'Escape') {
          backToReview();
        } else if (e.key === 'Enter') {
          confirmDelete();
        } else {
          return false;
        }
        return true;
      case 'finished':
      case 'failed':
        if (e.key !== 'Enter') {
          return false;
        }
        closeOverlay();
        return true;
    }
  }

  const planning = progressOf(job, 'plan', 0);
  const writing = progressOf(job, 'apply', plan?.ops.length ?? 0);
  const reviewing = (phase === 'reviewing' || phase === 'confirming') && plan !== null;

  return (
    <Dialog title="Sync" hints={hintsFor(phase, library && phase === 'choosing', plan?.ops.length === 0)} onKey={onKey}>
      {phase === 'choosing' && (
        <>
          <div className="ask-body">Where should the music be written?</div>
          <MenuList
            label="Where to write"
            rows={targets.map((t) => ({ marker: '', label: t.label, detail: t.path }))}
            cursor={cursor}
            onChoose={(i) => {
              if (i === cursor) {
                pickTarget();
              } else {
                chooseTarget(i);
              }
            }}
          />
          <Format library={library} how="w goes through them" />
        </>
      )}
      {phase === 'typing' && (
        <>
          <div className="ask-body">Write the music into which folder?</div>
          <div className="form">
            <label className="label">
              Folder
              <input
                ref={folderInput}
                className="edit"
                type="text"
                dir="auto"
                autoComplete="off"
                spellCheck={false}
                placeholder="~/Music/player"
                aria-label="Folder"
                value={folder}
                onChange={(e) => {
                  typeFolder(e.target.value);
                }}
              />
            </label>
          </div>
          <Format library={false} how="" />
          {error !== '' && <div className="bad">{error}</div>}
        </>
      )}
      {phase === 'planning' && (
        <>
          <div className="busy">
            Preparing <bdi dir="auto">{where}</bdi>…
          </div>
          {planning.total > 0 && <Progress done={planning.done} total={planning.total} label="Checked so far" />}
          <div className="ask-body">Checking every file against its record; esc cancels.</div>
        </>
      )}
      {reviewing && (
        <>
          {plan.ops.length === 0 ? (
            <div>
              <b>Nothing to do</b>
              <span className="dim">
                {' · '}
                <bdi dir="auto">{plan.where}</bdi> is up to date.
              </span>
            </div>
          ) : (
            <>
              <div>
                <b>{plan.text}</b>
                <span className="dim">{'  →  '}</span>
                <bdi dir="auto">{plan.where}</bdi>
              </div>
              <div ref={ops} className="ops" role="list" tabIndex={-1} aria-label="What would change">
                {plan.ops.map((op, i) => (
                  <div key={i} role="listitem" className={`op ${op.kind}`} title={op.path}>
                    <span className="g" role="img" aria-label={op.kind}>
                      {glyphs[op.kind]}
                    </span>
                    <bdi dir="auto">{op.path}</bdi>
                  </div>
                ))}
              </div>
              {plan.warnings.length > 0 && (
                <>
                  <div className="warn warns">{`${plural(plan.warnings.length, 'warning')} for the player`}</div>
                  {plan.warnings.slice(0, warningsShown).map((w, i) => (
                    <div key={i} className="dim">
                      <bdi dir="auto">{w.track}</bdi>
                      {'  '}
                      <bdi dir="auto">{w.message}</bdi>
                    </div>
                  ))}
                  {plan.warnings.length > warningsShown && <div className="dim">{`… ${String(plan.warnings.length - warningsShown)} more`}</div>}
                </>
              )}
              {plan.counts.stray > 0 && <div className="ask-body">Stray files are left alone. p also deletes the strays.</div>}
              {plan.counts.stray === 0 && plan.counts.remove > 0 && <div className="warn">The files marked − will be deleted. p keeps them.</div>}
              {phase === 'confirming' && (
                <div className="confirm">
                  <span className="warn">{`Type delete to remove ${plural(plan.counts.remove, 'file')}: `}</span>
                  <input
                    ref={confirmInput}
                    className="edit short"
                    type="text"
                    autoComplete="off"
                    spellCheck={false}
                    aria-label="Type delete to confirm"
                    value={confirm}
                    onChange={(e) => {
                      typeConfirm(e.target.value);
                    }}
                  />
                </div>
              )}
            </>
          )}
          {error !== '' && <div className="bad">{error}</div>}
          {phase === 'confirming' ? (
            <div className="buttons">
              <button type="button" className="btn primary" onClick={confirmDelete}>
                Delete and write
              </button>
              <button type="button" className="btn" onClick={backToReview}>
                Back
              </button>
            </div>
          ) : (
            <div className="buttons">
              <button type="button" className="btn primary" onClick={applyPlan}>
                Apply
              </button>
              {(plan.counts.stray > 0 || plan.prune) && (
                <button
                  type="button"
                  className="btn"
                  onClick={() => {
                    void togglePrune();
                  }}
                >
                  {plan.prune ? 'Keep the strays' : 'Delete the strays'}
                </button>
              )}
              <button type="button" className="btn" onClick={closeOverlay}>
                Cancel
              </button>
            </div>
          )}
        </>
      )}
      {phase === 'applying' && (
        <>
          <div className="busy">
            Writing <bdi dir="auto">{plan?.where}</bdi>…
          </div>
          <Progress done={writing.done} total={writing.total} label="Written so far" />
        </>
      )}
      {phase === 'finished' && (
        <>
          <div>
            <span className="good">Done</span> <bdi dir="auto">{plan?.where}</bdi>
          </div>
          <div className="ask-head">{summary}</div>
        </>
      )}
      {phase === 'failed' && (
        <div className="bad" role="alert">
          {error}
        </div>
      )}
    </Dialog>
  );
}
