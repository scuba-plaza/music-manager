import { Fragment, useEffect, useRef } from 'react';
import type { KeyEventLike } from '../keys/keymap';
import { Dialog } from '../overlay/Dialog';
import { MenuList } from '../overlay/MenuList';
import { closeOverlay } from '../overlay/open';
import {
  applyCandidate,
  cancelSearchEditing,
  chooseCandidate,
  editSearch,
  moveCandidate,
  searchFor,
  skipCover,
  useIdentify,
  type IdentifyPhase,
} from '../state/identify';
import { plural } from '../state/text';

const hintsFor: Record<IdentifyPhase, readonly { keys: string; label: string }[]> = {
  searching: [{ keys: 'esc', label: 'cancel' }],
  applying: [{ keys: 'esc', label: 'skip the cover' }],
  listing: [
    { keys: '↑↓', label: 'choose' },
    { keys: 'enter', label: 'apply' },
    { keys: '/', label: 'other words' },
    { keys: 'esc', label: 'close' },
  ],
  failed: [
    { keys: '/', label: 'other words' },
    { keys: 'esc', label: 'close' },
  ],
  editing: [
    { keys: 'enter', label: 'search' },
    { keys: 'tab', label: 'next' },
    { keys: 'esc', label: 'back' },
  ],
};

export function Identify() {
  const { phase, words, candidates, cursor, error } = useIdentify();
  const title = useRef<HTMLInputElement>(null);
  const artist = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (phase === 'editing') {
      title.current?.focus();
    }
  }, [phase]);

  function onKey(e: KeyEventLike): boolean {
    switch (phase) {
      case 'searching':
        return false;
      case 'applying':
        if (e.key === 'Escape') {
          skipCover();
          return true;
        }
        return false;
      case 'failed':
        if (e.key === 'q') {
          closeOverlay();
        } else if (e.key === '/') {
          editSearch();
        } else {
          return false;
        }
        return true;
      case 'editing':
        if (e.key === 'Escape') {
          cancelSearchEditing();
        } else if (e.key === 'Enter') {
          searchFor(title.current?.value ?? '', artist.current?.value ?? '');
        } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
          (document.activeElement === title.current ? artist.current : title.current)?.focus();
        } else {
          return false;
        }
        return true;
      case 'listing':
        if (e.key === 'q') {
          closeOverlay();
        } else if (e.key === 'ArrowUp' || e.key === 'k') {
          moveCandidate(-1);
        } else if (e.key === 'ArrowDown' || e.key === 'j') {
          moveCandidate(1);
        } else if (e.key === 'Home' || e.key === 'PageUp') {
          chooseCandidate(0);
        } else if (e.key === 'End' || e.key === 'PageDown') {
          chooseCandidate(candidates.length - 1);
        } else if (e.key === 'Enter') {
          void applyCandidate();
        } else if (e.key === '/') {
          editSearch();
        } else {
          return false;
        }
        return true;
    }
  }

  const quote = (
    <bdi dir="auto">{`“${words.title} – ${words.artist}”`}</bdi>
  );
  const chosen = candidates[cursor];

  return (
    <Dialog title="Identify" hints={hintsFor[phase]} onKey={onKey}>
      {phase === 'searching' && (
        <>
          <div className="busy">Searching MusicBrainz for {quote}…</div>
          <div className="ask-body">esc cancels</div>
        </>
      )}
      {phase === 'applying' && (
        <>
          <div className="busy">Applying…</div>
          <div className="ask-body">Fetching the cover can take a moment; esc skips it.</div>
        </>
      )}
      {phase === 'failed' && (
        <>
          <div className="warn" role="alert">
            <bdi dir="auto">{error}</bdi>
          </div>
          <div className="ask-body">Press / to search with other words.</div>
        </>
      )}
      {phase === 'editing' && (
        <>
          <div className="ask-body">Search for:</div>
          <div className="form">
            <label className="label">
              Title
              <input ref={title} className="edit" type="text" dir="auto" autoComplete="off" aria-label="title" defaultValue={words.title} />
            </label>
            <label className="label">
              Artist
              <input ref={artist} className="edit" type="text" dir="auto" autoComplete="off" aria-label="artist" defaultValue={words.artist} />
            </label>
          </div>
        </>
      )}
      {phase === 'listing' && chosen !== undefined && (
        <>
          <div className="ask-body">
            {plural(candidates.length, 'match')} for {quote}
          </div>
          <MenuList
            label="Matches"
            rows={candidates.map((c) => ({ marker: '', label: c.text, detail: '' }))}
            cursor={cursor}
            onChoose={(i) => {
              chooseCandidate(i);
            }}
          />
          <div className="ask-body">This would change:</div>
          <div className="diff">
            {chosen.diff.map((d) => (
              <Fragment key={d.label}>
                <span className="dim">{d.label}</span>
                {d.same ? (
                  <span className="dim">
                    <bdi dir="auto">{d.new}</bdi>
                    {'  (same)'}
                  </span>
                ) : (
                  <span>
                    <bdi dir="auto">{d.old}</bdi>
                    <span className="dim"> → </span>
                    <span className="accent">
                      <bdi dir="auto">{d.new}</bdi>
                    </span>
                  </span>
                )}
              </Fragment>
            ))}
          </div>
        </>
      )}
    </Dialog>
  );
}
