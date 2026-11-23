import { editHints, filterHints, hintsFor, type Hint } from '../keys/keymap';
import { hasSecondNumber } from '../state/inspector';
import { useApp, type AppState } from '../state/store';

function shownHints(state: AppState): readonly Hint[] {
  if (state.overlay !== null) {
    return state.dialogHints;
  }
  if (state.filterFocused) {
    return filterHints();
  }
  if (state.edit !== null) {
    return editHints(hasSecondNumber(state));
  }
  return hintsFor(state);
}

export function Footer() {
  const state = useApp();
  const { text, kind } = state.status;
  return (
    <footer className="footer">
      <div role="status" className={kind === '' ? 'status' : `status ${kind}`}>
        <bdi dir="auto">{text}</bdi>
      </div>
      <ul className="hints" aria-label="Keys">
        {shownHints(state).map((hint) => (
          <li key={`${hint.keys} ${hint.label}`} className="hint">
            <kbd>{hint.keys}</kbd> {hint.label}
          </li>
        ))}
      </ul>
    </footer>
  );
}
