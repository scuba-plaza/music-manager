import { memo, useEffect, useId, useLayoutEffect, useMemo, useState, type MouseEvent } from 'react';
import type { RowView } from '../api/types.gen';
import { filterBoxAttribute } from '../keys/dispatch';
import {
  composeFilter,
  foldRow,
  markRow,
  pointRow,
  run,
  setFilterFocused,
  takeTreeFocus,
  typeFilter,
} from '../state/actions';
import { useApp } from '../state/store';
import { isFiltering, isFlat, isFolder, isOpen, markStateOf, visibleRows, type MarkState } from '../state/tree';
import { scrollCursorIntoView } from '../state/treeDom';

const emptyLibrary = 'The library is empty. Press a to add from a link, or drop files on the page.';

const markGlyphs: Record<MarkState, string> = { none: '', some: '◐', all: '●' };
const checked: Record<MarkState, boolean | 'mixed' | undefined> = { none: undefined, some: 'mixed', all: true };

interface TreeRowProps {
  id: string;
  row: RowView;
  cursor: boolean;
  mark: MarkState;
  open: boolean;
  flat: boolean;
  filtering: boolean;
}

function clicked(e: MouseEvent, part: string): boolean {
  return e.target instanceof Element && e.target.closest(part) !== null;
}

const TreeRow = memo(function TreeRow({ id, row, cursor, mark, open, flat, filtering }: TreeRowProps) {
  const folder = isFolder(row);
  const classes = ['row', row.kind];
  if (cursor) {
    classes.push('cursor');
  }
  if (mark !== 'none') {
    classes.push('marked');
  }
  const right = folder ? String(row.count ?? 0) : (row.right ?? row.length ?? '');

  function click(e: MouseEvent): void {
    if (clicked(e, '.twist')) {
      foldRow(row.key);
    } else if (clicked(e, '.mark')) {
      markRow(row.key);
    } else {
      pointRow(row.key);
    }
  }

  function doubleClick(e: MouseEvent): void {
    if (!clicked(e, '.twist') && !clicked(e, '.mark')) {
      run('enterRow');
    }
  }

  return (
    <div
      role="treeitem"
      id={id}
      className={classes.join(' ')}
      data-key={row.key}
      data-mark={mark === 'none' ? undefined : mark}
      aria-level={row.depth + 1}
      aria-selected={cursor}
      aria-checked={checked[mark]}
      aria-expanded={folder ? open : undefined}
      style={flat ? undefined : { paddingInlineStart: `${String(row.depth * 1.25)}rem` }}
      onClick={click}
      onDoubleClick={doubleClick}
    >
      <span className="mark" aria-hidden="true">
        {markGlyphs[mark]}
      </span>
      <span className="twist" aria-hidden="true">
        {folder && !filtering ? (open ? '▾' : '▸') : ''}
      </span>
      {row.num !== undefined && <span className="num">{row.num}</span>}
      <span className="label">
        <bdi dir="auto">{row.label}</bdi>
      </span>
      {row.detail === undefined ? (
        <span className="spacer" />
      ) : (
        <span className="detail">
          <bdi dir="auto">{row.detail}</bdi>
        </span>
      )}
      <span className={folder ? 'right' : 'right mono'}>{right}</span>
    </div>
  );
});

function FilterBox() {
  const id = useId();
  const filter = useApp((s) => s.filter);
  const attribute = { [filterBoxAttribute]: '' };
  return (
    <div className="filterbar">
      <label htmlFor={id} className="sr-only">
        Filter
      </label>
      <span className="prompt" aria-hidden="true">
        /
      </span>
      <input
        {...attribute}
        id={id}
        className="filter"
        type="text"
        dir="auto"
        autoComplete="off"
        spellCheck={false}
        placeholder="filter  (words, status:unidentified, genre:folk)"
        value={filter}
        onChange={(e) => {
          typeFilter(e.target.value);
        }}
        onCompositionStart={() => {
          composeFilter(true);
        }}
        onCompositionEnd={() => {
          composeFilter(false);
        }}
        onFocus={() => {
          setFilterFocused(true);
        }}
        onBlur={() => {
          setFilterFocused(false);
        }}
      />
    </div>
  );
}

export function Tree() {
  const idBase = useId();
  const lib = useApp((s) => s.lib);
  const filter = useApp((s) => s.filter);
  const expanded = useApp((s) => s.expanded);
  const cursor = useApp((s) => s.cursor);
  const marks = useApp((s) => s.marks);
  const status = useApp((s) => s.status);
  const [failure, setFailure] = useState('');
  const shown = useMemo(() => visibleRows({ lib, filter, expanded }), [lib, filter, expanded]);
  const flat = isFlat(lib);
  const filtering = isFiltering(filter);

  useLayoutEffect(() => {
    scrollCursorIntoView();
  }, [shown, cursor]);

  useEffect(() => {
    if (lib !== null) {
      setFailure('');
    } else if (status.kind === 'bad') {
      setFailure(status.text);
    }
  }, [lib, status]);

  const activeAt = shown.findIndex((row) => row.key === cursor);
  const activeId = activeAt < 0 ? undefined : `${idBase}-${String(activeAt)}`;

  const failing = lib === null && failure !== '';
  let notice = '';
  if (lib === null) {
    notice = failure === '' ? 'Loading…' : failure;
  } else if (shown.length === 0) {
    if (lib.view.tracks === 0) {
      notice = emptyLibrary;
    } else {
      notice = filter.trim() === '' ? 'Loading…' : 'Nothing matches the filter.';
    }
  }

  return (
    <div className="pane-body tree-body" data-testid="tree-pane">
      <FilterBox />
      <div
        role="tree"
        aria-label="Library tree"
        aria-activedescendant={activeId}
        className="tree"
        tabIndex={0}
        onFocus={takeTreeFocus}
      >
        {shown.map((row, i) => (
          <TreeRow
            key={row.key}
            id={`${idBase}-${String(i)}`}
            row={row}
            cursor={row.key === cursor}
            mark={lib === null ? 'none' : markStateOf(lib.view.rows, row, marks)}
            open={isOpen({ filter, expanded }, row)}
            flat={flat}
            filtering={filtering}
          />
        ))}
      </div>
      {notice !== '' && (
        <div className="empty" role={failing ? 'alert' : 'status'}>
          <bdi dir="auto">{notice}</bdi>
        </div>
      )}
    </div>
  );
}
