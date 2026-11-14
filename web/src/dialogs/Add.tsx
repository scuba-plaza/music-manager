import { useEffect, useRef, useState, type ChangeEvent } from 'react';
import type { KeyEventLike } from '../keys/keymap';
import { Problem } from '../components/Problem';
import { Dialog } from '../overlay/Dialog';
import {
  chooseFiles,
  fetchLink,
  retryAdd,
  saveAdd,
  setField,
  showDuplicate,
  toggleMatch,
  typeLink,
  useAdd,
  type AddKey,
  type AddPhase,
} from '../state/add';

interface Field {
  key: AddKey;
  label: string;
}

const fields: readonly Field[] = [
  { key: 'title', label: 'Title' },
  { key: 'artist', label: 'Artist' },
  { key: 'album', label: 'Album' },
  { key: 'track', label: 'Track' },
  { key: 'of', label: 'Tracks on album' },
  { key: 'released', label: 'Released' },
  { key: 'genre', label: 'Genre' },
  { key: 'cutStart', label: 'Cut start' },
  { key: 'cutEnd', label: 'Cut end' },
];

const hintsFor: Record<AddPhase, readonly { keys: string; label: string }[]> = {
  asking: [
    { keys: 'enter', label: 'fetch' },
    { keys: 'esc', label: 'close' },
  ],
  fetching: [{ keys: 'esc', label: 'cancel' }],
  saving: [{ keys: 'esc', label: 'close' }],
  duplicate: [
    { keys: 'enter', label: 'show it' },
    { keys: 'esc', label: 'close' },
  ],
  failed: [
    { keys: 'enter', label: 'tries again' },
    { keys: 'esc', label: 'close' },
  ],
  form: [
    { keys: 'enter', label: 'add' },
    { keys: 'tab', label: 'next field' },
    { keys: 'esc', label: 'close' },
  ],
};

function Elapsed({ since }: { since: number }) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = setInterval(() => {
      setNow(Date.now());
    }, 1000);
    return () => {
      clearInterval(timer);
    };
  }, []);
  return <>{`${String(Math.floor((now - since) / 1000))}s`}</>;
}

export function Add() {
  const { phase, link, error, started, draft, useMatch, form, existing, pathNote } = useAdd();
  const linkInput = useRef<HTMLInputElement>(null);
  const files = useRef<HTMLInputElement>(null);
  const grid = useRef<HTMLDivElement>(null);
  const focusAt = useRef(0);

  function inputs(): HTMLInputElement[] {
    return [...(grid.current?.querySelectorAll('input') ?? [])];
  }

  useEffect(() => {
    if (phase === 'asking') {
      linkInput.current?.focus();
    } else if (phase === 'form') {
      inputs()[focusAt.current]?.focus();
    }
  }, [phase]);

  function focusBy(by: number): void {
    const all = inputs();
    const at = Math.max(
      0,
      all.findIndex((input) => input === document.activeElement),
    );
    all[(at + by + all.length) % all.length]?.focus();
  }

  function onKey(e: KeyEventLike): boolean {
    switch (phase) {
      case 'asking':
        if (e.key !== 'Enter') {
          return false;
        }
        fetchLink();
        return true;
      case 'duplicate':
        if (e.key !== 'Enter') {
          return false;
        }
        showDuplicate();
        return true;
      case 'failed':
        if (e.key !== 'Enter') {
          return false;
        }
        retryAdd();
        return true;
      case 'form':
        if (e.key === 'Enter') {
          void saveAdd();
        } else if (e.key === 'F2') {
          toggleMatch();
        } else if (e.key === 'Tab') {
          focusBy(e.shiftKey ? -1 : 1);
        } else if (e.key === 'ArrowDown') {
          focusBy(1);
        } else if (e.key === 'ArrowUp') {
          focusBy(-1);
        } else {
          return false;
        }
        return true;
      case 'fetching':
      case 'saving':
        return false;
    }
  }

  function chosen(e: ChangeEvent<HTMLInputElement>): void {
    const list = [...(e.target.files ?? [])];
    chooseFiles(list);
  }

  const match = draft?.match;
  const hints =
    phase === 'form' && match !== undefined
      ? [...hintsFor.form.slice(0, 2), { keys: 'F2', label: useMatch ? 'ignore the match' : 'use the match' }, ...hintsFor.form.slice(2)]
      : hintsFor[phase];

  return (
    <Dialog title="Add from YouTube" hints={hints} onKey={onKey}>
      {phase === 'asking' && (
        <>
          <div className="ask-body">Paste a link to a video or a track:</div>
          <div className="form">
            <input
              ref={linkInput}
              className="edit"
              type="text"
              autoComplete="off"
              spellCheck={false}
              placeholder="https://www.youtube.com/watch?v=…"
              aria-label="Link"
              value={link}
              onChange={(e) => {
                typeLink(e.target.value);
              }}
            />
          </div>
          {error !== '' && <Problem text={error} />}
          <div className="buttons">
            <button
              type="button"
              className="btn"
              onClick={() => {
                files.current?.click();
              }}
            >
              Choose MP3 files instead…
            </button>
            <input ref={files} type="file" accept=".mp3,audio/mpeg" multiple className="sr-only" tabIndex={-1} aria-label="MP3 files" onChange={chosen} />
          </div>
          <div className="ask-body">MP3 files can also be dropped on the page.</div>
        </>
      )}
      {phase === 'fetching' && (
        <>
          <div className="busy">
            Downloading… <Elapsed since={started} />
          </div>
          <div className="ask-body">This also looks the track up on MusicBrainz. esc cancels.</div>
        </>
      )}
      {phase === 'saving' && <div className="busy">Adding to the library…</div>}
      {phase === 'duplicate' && existing !== null && (
        <>
          <div className="warn">This audio is already in your library:</div>
          <div className="ask-head">
            <bdi dir="auto">{existing.title}</bdi>
            <span className="dim">
              {' · '}
              <bdi dir="auto">{existing.artist}</bdi>
            </span>
          </div>
        </>
      )}
      {phase === 'failed' && (
        <Problem text={error} />
      )}
      {phase === 'form' && (
        <>
          {match === undefined ? (
            <div className="dim">No match on MusicBrainz: these are YouTube’s words.</div>
          ) : useMatch ? (
            <div className="accent">
              <bdi dir="auto">{`Matched on MusicBrainz: ${match.artist} - ${match.title} · ${match.album}`}</bdi>
            </div>
          ) : (
            <div className="dim">Ignoring the match: these are YouTube’s words.</div>
          )}
          <div
            ref={grid}
            className="form"
            onFocus={(e) => {
              focusAt.current = Math.max(
                0,
                inputs().findIndex((input) => input === e.target),
              );
            }}
          >
            {fields
              .filter((f) => f.key !== 'of')
              .map((f) => (
                <label key={f.key} className="label">
                  {f.label}
                  {f.key === 'track' ? (
                    <span className="pair">
                      <input
                        className="edit"
                        type="text"
                        inputMode="numeric"
                        autoComplete="off"
                        aria-label="Track"
                        value={form.track}
                        onChange={(e) => {
                          setField('track', e.target.value);
                        }}
                      />
                      <span className="dim"> of </span>
                      <input
                        className="edit short"
                        type="text"
                        inputMode="numeric"
                        autoComplete="off"
                        aria-label="Tracks on album"
                        value={form.of}
                        onChange={(e) => {
                          setField('of', e.target.value);
                        }}
                      />
                    </span>
                  ) : (
                    <input
                      className="edit"
                      type="text"
                      dir="auto"
                      autoComplete="off"
                      spellCheck={false}
                      aria-label={f.label}
                      value={form[f.key]}
                      onChange={(e) => {
                        setField(f.key, e.target.value);
                      }}
                    />
                  )}
                </label>
              ))}
          </div>
          <div className="pathrow accent">
            {pathNote !== '' && <span aria-hidden="true">→ </span>}
            <span className="pathline">
              <bdi dir="auto">{pathNote}</bdi>
            </span>
          </div>
          {error !== '' && <Problem text={error} />}
        </>
      )}
    </Dialog>
  );
}
