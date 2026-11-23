import { useEffect, useRef, useState, type ChangeEvent } from 'react';
import type { KeyEventLike } from '../keys/keymap';
import { FormField } from '../components/FormField';
import { Problem } from '../components/Problem';
import { Dialog } from '../overlay/Dialog';
import { ringKey } from '../overlay/focusRing';
import {
  addOrder,
  chooseDraftCover,
  chooseFiles,
  fetchLink,
  leaveField,
  removeDraftCover,
  saveAdd,
  setField,
  showDuplicate,
  typeLink,
  useAdd,
  type AddKey,
  type AddPhase,
} from '../state/add';
import { shownErrors } from '../state/form';

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
  const { phase, link, error, linkError, checks, started, form, existing, pathNote, draft, cover, coverError } = useAdd();
  const errors = shownErrors(checks);
  const linkInput = useRef<HTMLInputElement>(null);
  const files = useRef<HTMLInputElement>(null);
  const picture = useRef<HTMLInputElement>(null);
  const grid = useRef<HTMLDivElement>(null);
  const whole = useRef<HTMLDivElement>(null);
  const focusAt = useRef(0);

  function inputs(): HTMLInputElement[] {
    return [...(grid.current?.querySelectorAll('input') ?? [])];
  }

  function controls(): HTMLElement[] {
    return [...(whole.current?.querySelectorAll<HTMLElement>('.form input, .coverrow button') ?? [])];
  }

  useEffect(() => {
    if (phase === 'asking') {
      linkInput.current?.focus();
    } else if (phase === 'form') {
      inputs()[focusAt.current]?.focus();
    }
  }, [phase]);

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
        fetchLink();
        return true;
      case 'form':
        if (e.key === 'Enter' && document.activeElement instanceof HTMLButtonElement) {
          return false;
        }
        if (e.key === 'Enter') {
          void saveAdd().then((wrong) => {
            if (wrong !== undefined) {
              inputs()[addOrder.indexOf(wrong)]?.focus();
            }
          });
          return true;
        }
        return ringKey(e, controls());
      case 'fetching':
      case 'saving':
        return false;
    }
  }

  function chosen(e: ChangeEvent<HTMLInputElement>): void {
    const list = [...(e.target.files ?? [])];
    chooseFiles(list);
  }

  function pictured(e: ChangeEvent<HTMLInputElement>): void {
    const [first] = [...(e.target.files ?? [])];
    e.target.value = '';
    if (first !== undefined) {
      void chooseDraftCover(first);
    }
  }

  return (
    <Dialog title="Add from YouTube" hints={hintsFor[phase]} onKey={onKey}>
      {phase === 'asking' && (
        <>
          <div className="ask-body">Paste a link to a video or a track:</div>
          <div className="form">
            <FormField name="link" label="Link" error={linkError} hideLabel>
              {(control) => (
                <input
                  {...control}
                  ref={linkInput}
                  className="edit"
                  type="text"
                  autoComplete="off"
                  spellCheck={false}
                  placeholder="https://www.youtube.com/watch?v=…"
                  value={link}
                  onChange={(e) => {
                    typeLink(e.target.value);
                  }}
                />
              )}
            </FormField>
          </div>
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
          <div className="ask-body">esc cancels.</div>
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
          <div className="dim">{draft?.note ?? ''}</div>
          <div ref={whole}>
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
              .map((f) =>
                f.key === 'track' ? (
                  <FormField key={f.key} name={f.key} label={f.label} error={[errors.track, errors.of].filter((m) => m !== undefined).join(' ')}>
                    {(control) => (
                      <span className="pair">
                        <input
                          {...control}
                          className="edit"
                          type="text"
                          inputMode="numeric"
                          autoComplete="off"
                          aria-invalid={errors.track === undefined ? undefined : true}
                          value={form.track}
                          onChange={(e) => {
                            setField('track', e.target.value);
                          }}
                          onBlur={() => {
                            leaveField('track');
                          }}
                        />
                        <span className="dim"> of </span>
                        <input
                          className="edit short"
                          type="text"
                          inputMode="numeric"
                          autoComplete="off"
                          aria-label="Tracks on album"
                          aria-invalid={errors.of === undefined ? undefined : true}
                          aria-describedby={control['aria-describedby']}
                          value={form.of}
                          onChange={(e) => {
                            setField('of', e.target.value);
                          }}
                          onBlur={() => {
                            leaveField('of');
                          }}
                        />
                      </span>
                    )}
                  </FormField>
                ) : (
                  <FormField key={f.key} name={f.key} label={f.label} error={errors[f.key]}>
                    {(control) => (
                      <input
                        {...control}
                        className="edit"
                        type="text"
                        dir="auto"
                        autoComplete="off"
                        spellCheck={false}
                        value={form[f.key]}
                        onChange={(e) => {
                          setField(f.key, e.target.value);
                        }}
                        onBlur={() => {
                          leaveField(f.key);
                        }}
                      />
                    )}
                  </FormField>
                ),
              )}
          </div>
          <div className="coverrow">
            {cover !== '' && draft !== null && (
              <div className="cover-box">
                <img src={`/api/add/${encodeURIComponent(draft.draft)}/cover?v=${cover.slice(0, 16)}`} alt="Cover" />
              </div>
            )}
            <div className="buttons">
              <button
                type="button"
                className="btn"
                onClick={() => {
                  picture.current?.click();
                }}
              >
                {cover === '' ? 'Add a cover…' : 'Replace cover…'}
              </button>
              {cover !== '' && (
                <button
                  type="button"
                  className="btn"
                  onClick={() => {
                    void removeDraftCover();
                  }}
                >
                  Remove cover
                </button>
              )}
              <input ref={picture} type="file" accept="image/jpeg,image/png" className="sr-only" tabIndex={-1} aria-label="Cover picture" onChange={pictured} />
            </div>
            {coverError !== '' && <Problem text={coverError} />}
          </div>
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
