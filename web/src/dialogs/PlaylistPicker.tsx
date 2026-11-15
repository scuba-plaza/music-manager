import { useEffect, useRef, useState } from 'react';
import type { PlaylistView } from '../api/types.gen';
import { motionOf, moveTo } from '../keys/dialog';
import type { KeyEventLike } from '../keys/keymap';
import { FormField } from '../components/FormField';
import { Problem } from '../components/Problem';
import { Dialog } from '../overlay/Dialog';
import { MenuList } from '../overlay/MenuList';
import { addToPlaylist } from '../overlay/confirm';
import { closeOverlay } from '../state/store';
import { plural } from '../state/text';

interface PlaylistPickerProps {
  ids: readonly string[];
  lists: readonly PlaylistView[];
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
    const motion = motionOf(e);
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
    } else if (motion !== undefined) {
      setCursor(moveTo(entries, cursor, motion));
    } else if (e.key === 'c') {
      name(true);
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
        { keys: 'c', label: 'new' },
        { keys: 'esc', label: 'close' },
      ];
  const rows = [...lists.map((p) => ({ marker: '', label: p.name, detail: String(p.count) })), { marker: '', label: 'New playlist…', detail: '' }];

  return (
    <Dialog title="Add to a playlist" hints={hints} onKey={onKey}>
      <div className="ask-body">{`Adding ${plural(ids.length, 'track')}`}</div>
      {naming ? (
        <FormField name="name" label="Name of the playlist" error={error} hideLabel>
          {(control) => (
            <input
              {...control}
              ref={input}
              className="edit"
              type="text"
              dir="auto"
              maxLength={80}
              autoComplete="off"
              spellCheck={false}
              placeholder="name of the playlist"
              value={typed}
              onChange={(e) => {
                setTyped(e.target.value);
                setError('');
              }}
            />
          )}
        </FormField>
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
      {!naming && error !== '' && <Problem text={error} />}
    </Dialog>
  );
}
