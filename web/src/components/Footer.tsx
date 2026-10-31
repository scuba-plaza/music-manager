import { hintsFor } from '../keys/keymap';
import { useApp } from '../state/store';

export function Footer() {
  const state = useApp();
  const { text, kind } = state.status;
  return (
    <footer className="footer">
      <div role="status" className={kind === '' ? 'status' : `status ${kind}`}>
        {text}
      </div>
      <ul className="hints" aria-label="Keys">
        {hintsFor(state).map((hint) => (
          <li key={`${hint.keys} ${hint.label}`} className="hint">
            <kbd>{hint.keys}</kbd> {hint.label}
          </li>
        ))}
      </ul>
    </footer>
  );
}
