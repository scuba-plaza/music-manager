import { useEffect, useRef, useState } from 'react';
import type { PlaylistView } from '../api/types.gen';
import type { KeyEventLike } from '../keys/keymap';
import { Dialog } from '../overlay/Dialog';
import { MenuList, moveIn } from '../overlay/MenuList';
import { addToPlaylist, closeOverlay } from '../overlay/open';

interface PlaylistPickerProps {
  ids: readonly string[];
  lists: readonly PlaylistView[];
}

function tracks(n: number): string {
  return n === 1 ? '1 track' : `${String(n)} tracks`;
}

export function PlaylistPicker({ ids, lists }: PlaylistPickerProps) {
  const [cursor, setCursor] = useState(0);
  const [naming, setNaming] = useState(lists.length === 0);
  const [typed, setTyped] = useState('');
  const [error, setError] = useState('');
  const input = useRef<HTMLInputElement>(null);
  const entries = lists.length + 1;

  useEffect(() => {
    if (naming) {
      input.current?.focus();
    }
  }, [naming]);

  function name(isNaming: boolean): void {
    setNaming(isNaming);
    setError('');
  }

  async function submit(at: number): Promise<void> {
    const chosen = naming ? typed.trim() : (lists[at]?.name ?? '');
    if (chosen === '') {
      setError('Type a name');
      return;
    }
    setError(await addToPlaylist(chosen, naming, ids));
  }

  function pick(at: number): void {
    if (!naming && at === lists.length) {
      name(true);
    } else {
      void submit(at);
    }
  }

  function onKey(e: KeyEventLike): boolean {
    if (e.key === 'Escape') {
      if (naming && lists.length > 0) {
        name(false);
      } else {
        closeOverlay();
      }
    } else if (e.key === 'Enter') {
      pick(cursor);
    } else if (naming) {
      return false;
    } else if (e.key === 'ArrowUp' || e.key === 'k') {
      setCursor(moveIn(entries, cursor, -1));
    } else if (e.key === 'ArrowDown' || e.key === 'j') {
      setCursor(moveIn(entries, cursor, 1));
    } else if (e.key === 'n') {
      name(true);
    } else if (e.key === 'q') {
      closeOverlay();
    } else {
      return false;
    }
    return true;
  }

  const hints = naming
    ? [
        { keys: 'enter', label: 'make it' },
        { keys: 'esc', label: lists.length > 0 ? 'back' : 'close' },
      ]
    : [
        { keys: 'enter', label: 'add' },
        { keys: 'n', label: 'new' },
        { keys: 'esc', label: 'close' },
      ];
  const rows = [...lists.map((p) => ({ marker: '', label: p.name, detail: String(p.count) })), { marker: '', label: 'New playlist…', detail: '' }];

  return (
    <Dialog title="Add to a playlist" hints={hints} onKey={onKey}>
      <div className="ask-body">{`Adding ${tracks(ids.length)}`}</div>
      {naming ? (
        <input
          ref={input}
          className="edit"
          type="text"
          dir="auto"
          maxLength={80}
          autoComplete="off"
          spellCheck={false}
          placeholder="name of the playlist"
          aria-label="Name of the playlist"
          value={typed}
          onChange={(e) => {
            setTyped(e.target.value);
            setError('');
          }}
        />
      ) : (
        <MenuList
          label="Playlists"
          rows={rows}
          cursor={cursor}
          onChoose={(i) => {
            setCursor(i);
            pick(i);
          }}
        />
      )}
      {error !== '' && (
        <div className="bad" role="alert">
          {error}
        </div>
      )}
    </Dialog>
  );
}
