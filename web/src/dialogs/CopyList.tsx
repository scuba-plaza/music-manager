import { useState } from 'react';
import type { CopyChoice } from '../api/types.gen';
import type { KeyEventLike } from '../keys/keymap';
import { Dialog } from '../overlay/Dialog';
import { MenuList, moveIn } from '../overlay/MenuList';
import { closeOverlay, copyAttributes } from '../overlay/open';

interface CopyListProps {
  id: string;
  title: string;
  choices: readonly CopyChoice[];
}

const hints = [
  { keys: 'space', label: 'ticks' },
  { keys: 'enter', label: 'copies' },
  { keys: 'esc', label: 'close' },
];

export function CopyList({ id, title, choices }: CopyListProps) {
  const [ticked, setTicked] = useState(() => choices.map((c) => c.has));
  const [cursor, setCursor] = useState(() => Math.max(0, choices.findIndex((c) => c.has)));
  const [error, setError] = useState('');

  function toggle(i: number): void {
    if (choices[i]?.has !== true) {
      return;
    }
    setTicked(ticked.map((on, j) => (j === i ? !on : on)));
    setError('');
  }

  async function submit(): Promise<void> {
    const attrs = choices.filter((_, i) => ticked[i] === true).map((c) => c.key);
    if (attrs.length === 0) {
      setError('Tick what to copy');
      return;
    }
    setError(await copyAttributes(id, attrs));
  }

  function onKey(e: KeyEventLike): boolean {
    if (e.key === 'q') {
      closeOverlay();
    } else if (e.key === 'ArrowUp' || e.key === 'k') {
      setCursor(moveIn(choices.length, cursor, -1));
    } else if (e.key === 'ArrowDown' || e.key === 'j') {
      setCursor(moveIn(choices.length, cursor, 1));
    } else if (e.key === ' ') {
      toggle(cursor);
    } else if (e.key === 'Enter') {
      void submit();
    } else {
      return false;
    }
    return true;
  }

  const rows = choices.map((c, i) => ({
    marker: c.has ? (ticked[i] === true ? '[x]' : '[ ]') : '',
    label: c.label,
    detail: c.has ? c.text : '—',
    off: !c.has,
  }));

  return (
    <Dialog
      title={
        <>
          Copy from <bdi dir="auto">{title}</bdi>
        </>
      }
      hints={hints}
      onKey={onKey}
    >
      <MenuList
        label="What to copy"
        rows={rows}
        cursor={cursor}
        onChoose={(i) => {
          setCursor(i);
          toggle(i);
        }}
      />
      {error !== '' && (
        <div className="bad" role="alert">
          {error}
        </div>
      )}
    </Dialog>
  );
}
