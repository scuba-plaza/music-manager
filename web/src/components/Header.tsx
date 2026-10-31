import { useEffect, useLayoutEffect, useState } from 'react';
import { recall, remember, useApp } from '../state/store';

type Theme = 'dark' | 'light';

const themeKey = 'mm.theme';

function recallTheme(): Theme {
  return recall(themeKey) === 'light' ? 'light' : 'dark';
}

function count(n: number, noun: string): string {
  return `${String(n)} ${noun}${n === 1 ? '' : 's'}`;
}

export function Header() {
  const root = useApp((s) => s.info?.root);
  const lib = useApp((s) => s.lib);
  const marked = useApp((s) => s.marks.size);
  const [theme, setTheme] = useState(recallTheme);

  useLayoutEffect(() => {
    document.documentElement.dataset.theme = theme;
  }, [theme]);

  useEffect(() => {
    document.title = root === undefined ? 'mm' : `mm · ${root}`;
  }, [root]);

  function toggleTheme(): void {
    const next = theme === 'light' ? 'dark' : 'light';
    setTheme(next);
    remember(themeKey, next);
  }

  const parts = lib ? [count(lib.view.tracks, 'track')] : [];
  if (lib && lib.view.needWork > 0) {
    parts.push(`${String(lib.view.needWork)} need work`);
  }
  if (marked > 0) {
    parts.push(`${String(marked)} marked`);
  }
  const waiting = lib?.unsynced.n ?? 0;

  return (
    <header className="header">
      <div className="head-start">
        <span className="brand">mm</span>
        {root !== undefined && (
          <span className="root" title={root}>
            <bdi dir="auto">{root}</bdi>
          </span>
        )}
      </div>
      <div className="head-end">
        {lib && <span className="summary">{parts.join(' · ')}</span>}
        {waiting > 0 && (
          <button type="button" className="chip">
            {`${String(waiting)} to sync · press s`}
          </button>
        )}
        <button
          type="button"
          className="chip plain"
          title="Switch between the dark and the light theme"
          aria-label="Switch theme"
          onClick={toggleTheme}
        >
          ◐
        </button>
      </div>
    </header>
  );
}
