import { answerOf } from '../keys/dialog';
import type { KeyEventLike } from '../keys/keymap';
import { Dialog } from '../overlay/Dialog';
import { closeOverlay } from '../state/store';

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
    const answer = answerOf(e);
    if (answer === 'yes') {
      go();
    } else if (answer === 'no') {
      closeOverlay();
    } else {
      return false;
    }
    return true;
  }

  return (
    <Dialog
      title="Are you sure?"
      hints={[
        { keys: 'y', label: yes },
        { keys: 'n', label: 'no' },
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
