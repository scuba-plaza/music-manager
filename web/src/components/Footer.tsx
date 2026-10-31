import { editHints, filterHints, hintsFor } from '../keys/keymap';
import { hasSecondNumber } from '../state/inspector';
import { useApp } from '../state/store';

export function Footer() {
  const state = useApp();
  const { text, kind } = state.status;
  let hints = hintsFor(state);
  if (state.filterFocused) {
    hints = filterHints();
  } else if (state.edit !== null) {
    hints = editHints(hasSecondNumber(state));
  }
  return (
    <footer className="footer">
      <div role="status" className={kind === '' ? 'status' : `status ${kind}`}>
        {text}
      </div>
      <ul className="hints" aria-label="Keys">
        {hints.map((hint) => (
          <li key={`${hint.keys} ${hint.label}`} className="hint">
            <kbd>{hint.keys}</kbd> {hint.label}
          </li>
        ))}
      </ul>
    </footer>
  );
}
