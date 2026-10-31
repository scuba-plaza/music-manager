import { Fragment, useState } from 'react';
import { helpPages, type KeyEventLike } from '../keys/keymap';
import { Dialog } from '../overlay/Dialog';
import { closeOverlay } from '../overlay/open';

const pages = helpPages();
const closers = new Set(['Escape', '?', 'q', 'Enter']);
const forward = new Set(['ArrowRight', 'l']);
const backward = new Set(['ArrowLeft', 'h']);
const hints = [
  { keys: '←→', label: 'page' },
  { keys: 'esc', label: 'close' },
];

let lastPage = 0;

export function Help() {
  const [page, setPage] = useState(lastPage);
  const rows = pages[page]?.rows ?? [];

  function turn(to: number): void {
    lastPage = (to + pages.length) % pages.length;
    setPage(lastPage);
  }

  function onKey(e: KeyEventLike): boolean {
    const n = Number(e.key);
    if (closers.has(e.key)) {
      closeOverlay();
    } else if (forward.has(e.key) || (e.key === 'Tab' && !e.shiftKey)) {
      turn(page + 1);
    } else if (backward.has(e.key) || e.key === 'Tab') {
      turn(page - 1);
    } else if (/^[1-9]$/.test(e.key) && n <= pages.length) {
      turn(n - 1);
    } else {
      return false;
    }
    return true;
  }

  return (
    <Dialog title="Keys" hints={hints} onKey={onKey}>
      <div className="tabs" role="tablist" aria-label="Pages">
        {pages.map((p, i) => (
          <button
            key={p.page}
            type="button"
            role="tab"
            tabIndex={-1}
            className={i === page ? 'tab on' : 'tab'}
            aria-selected={i === page}
            onClick={() => {
              turn(i);
            }}
          >
            {`${String(i + 1)} ${p.page}`}
          </button>
        ))}
      </div>
      <dl className="keys" role="tabpanel" aria-label={pages[page]?.page}>
        {rows.map((row) => (
          <Fragment key={`${row.keys} ${row.text}`}>
            <dt>{row.keys}</dt>
            <dd>{row.text}</dd>
          </Fragment>
        ))}
      </dl>
      <p className="dialog-note">Ctrl+C in the terminal where mm runs stops the program.</p>
      <div className="dialog-foot">{`← → or 1-${String(pages.length)} turn the page · esc closes`}</div>
    </Dialog>
  );
}
