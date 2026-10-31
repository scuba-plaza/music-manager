import type { KeyEventLike } from '../keys/keymap';
import { Dialog } from '../overlay/Dialog';
import { closeOverlay } from '../overlay/open';

interface AskProps {
  head: string;
  body: readonly string[];
  yes: string;
  onYes: () => void;
}

export function Ask({ head, body, yes, onYes }: AskProps) {
  function go(): void {
    closeOverlay();
    onYes();
  }

  function onKey(e: KeyEventLike): boolean {
    if (e.key === 'y' || e.key === 'Enter') {
      go();
      return true;
    }
    if (e.key === 'n' || e.key === 'q') {
      closeOverlay();
      return true;
    }
    return false;
  }

  return (
    <Dialog
      title="Are you sure?"
      hints={[
        { keys: 'y/enter', label: yes },
        { keys: 'esc', label: 'close' },
      ]}
      onKey={onKey}
    >
      <div className="ask-head">
        <bdi dir="auto">{head}</bdi>
      </div>
      {body.map((line, i) => (
        <div key={i} className={line.startsWith('•') ? 'ask-item' : 'ask-body'}>
          <bdi dir="auto">{line}</bdi>
        </div>
      ))}
      <div className="buttons">
        <button type="button" className="btn primary" onClick={go}>
          {yes}
        </button>
        <button type="button" className="btn" onClick={closeOverlay}>
          Cancel
        </button>
      </div>
    </Dialog>
  );
}
