import { useState } from 'react';
import type { SortView } from '../api/types.gen';
import type { KeyEventLike } from '../keys/keymap';
import { Dialog } from '../overlay/Dialog';
import { MenuList, moveIn } from '../overlay/MenuList';
import { chooseSort, closeOverlay } from '../overlay/open';

const hints = [
  { keys: 'enter', label: 'choose' },
  { keys: 'r', label: 'reverse' },
  { keys: 'esc', label: 'close' },
];

export function SortMenu({ sort }: { sort: SortView }) {
  const choices = sort.choices;
  const [cursor, setCursor] = useState(() => Math.max(0, choices.findIndex((c) => c.key === sort.key)));
  const [reverse, setReverse] = useState(sort.reverse);

  function pick(i: number): void {
    const choice = choices[i];
    if (choice === undefined) {
      return;
    }
    closeOverlay();
    void chooseSort(choice.key, choice.key !== '' && reverse);
  }

  function onKey(e: KeyEventLike): boolean {
    const n = Number(e.key);
    if (e.key === 'q' || e.key === 'S') {
      closeOverlay();
    } else if (e.key === 'ArrowUp' || e.key === 'k') {
      setCursor(moveIn(choices.length, cursor, -1));
    } else if (e.key === 'ArrowDown' || e.key === 'j') {
      setCursor(moveIn(choices.length, cursor, 1));
    } else if (e.key === 'r') {
      setReverse(!reverse);
    } else if (e.key === 'Enter') {
      pick(cursor);
    } else if (/^[1-9]$/.test(e.key) && n <= choices.length) {
      pick(n - 1);
    } else {
      return false;
    }
    return true;
  }

  const rows = choices.map((c, i) => ({
    marker: String(i + 1),
    label: c.label,
    detail: c.key !== '' && reverse ? c.backward : c.forward,
  }));

  return (
    <Dialog title="Sort" hints={hints} onKey={onKey}>
      <MenuList label="Orders" rows={rows} cursor={cursor} onChoose={pick} />
      <div className="dialog-foot">
        <kbd>r</kbd> reverse: {reverse ? 'on' : 'off'}
      </div>
    </Dialog>
  );
}
