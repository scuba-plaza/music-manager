import { describe, expect, it } from 'vitest';
import type { RowView } from '../api/types.gen';
import { sampleLibrary, sampleRows } from '../testing/fakes';
import { ancestorKeys, idsOf, isFlat, isOpen, keysToOpenOneLevel, markStateOf, settledCursor, visibleRows } from './tree';

const sep = '\u001f';
const milesKey = `artist${sep}Miles Davis`;
const kindOfBlueKey = `album${sep}Miles Davis${sep}Kind of Blue`;
const lateNightKey = `playlist${sep}Late night`;

function view(overrides: { filter?: string; expanded?: string[]; sortKey?: string } = {}) {
  const lib = sampleLibrary();
  if (overrides.sortKey !== undefined) {
    lib.view.sort = { ...lib.view.sort, key: overrides.sortKey };
  }
  return { lib, filter: overrides.filter ?? '', expanded: new Set(overrides.expanded ?? []) };
}

function labels(rows: readonly RowView[]): string[] {
  return rows.map((row) => row.label);
}

describe('visibleRows', () => {
  it('shows only the top level while nothing is open', () => {
    expect(labels(visibleRows(view()))).toEqual(['Playlists', 'Needs work', 'فيروز', 'John Coltrane', 'Miles Davis', 'Nina Simone']);
  });

  it('shows what is inside the open rows, and only those', () => {
    const rows = visibleRows(view({ expanded: [milesKey] }));

    expect(labels(rows)).toEqual(['Playlists', 'Needs work', 'فيروز', 'John Coltrane', 'Miles Davis', 'Kind of Blue', 'Nina Simone']);
  });

  it('hides the tracks of an album that is closed inside an open artist', () => {
    const rows = visibleRows(view({ expanded: [milesKey, kindOfBlueKey] }));

    expect(labels(rows).slice(4, 9)).toEqual(['Miles Davis', 'Kind of Blue', 'So What', 'Blue in Green', 'Nina Simone']);
  });

  it('shows every row while a filter is typed, whatever is open', () => {
    expect(visibleRows(view({ filter: 'miles' }))).toHaveLength(sampleRows().length);
  });

  it('does not count a filter of blanks as a filter', () => {
    expect(labels(visibleRows(view({ filter: '   ' }))).length).toBe(6);
  });

  it('shows every row of a sorted list', () => {
    expect(visibleRows(view({ sortKey: 'title' }))).toHaveLength(sampleRows().length);
  });

  it('shows nothing before the library has loaded', () => {
    expect(visibleRows({ lib: null, filter: '', expanded: new Set() })).toEqual([]);
  });
});

describe('isFlat and isOpen', () => {
  it('a library with a sort key is a flat list', () => {
    expect(isFlat(view({ sortKey: 'title' }).lib)).toBe(true);
    expect(isFlat(view().lib)).toBe(false);
    expect(isFlat(null)).toBe(false);
  });

  it('a row is open when it is in expanded or a filter is typed', () => {
    const miles = sampleRows().find((row) => row.key === milesKey);
    if (!miles) {
      throw new Error('the sample has no Miles Davis');
    }

    expect(isOpen({ filter: '', expanded: new Set() }, miles)).toBe(false);
    expect(isOpen({ filter: '', expanded: new Set([milesKey]) }, miles)).toBe(true);
    expect(isOpen({ filter: 'x', expanded: new Set() }, miles)).toBe(true);
  });
});

describe('idsOf', () => {
  const rows = sampleRows();
  const rowOf = (key: string) => {
    const found = rows.find((row) => row.key === key);
    if (!found) {
      throw new Error(`the sample has no row ${key}`);
    }
    return found;
  };

  it('a track stands for itself', () => {
    expect(idsOf(rows, rowOf(`track${sep}sowhat`))).toEqual(['sowhat']);
  });

  it('an artist stands for the tracks of its albums, in order and once each', () => {
    expect(idsOf(rows, rowOf(milesKey))).toEqual(['sowhat', 'blue']);
  });

  it('a playlist stands for the tracks in it', () => {
    expect(idsOf(rows, rowOf(lateNightKey))).toEqual(['blue']);
  });

  it('the Playlists group counts a track in two playlists once', () => {
    const doubled = rows.flatMap((row) => (row.key.startsWith('pltrack') ? [row, { ...row, key: `${row.key}-again` }] : [row]));
    const group = doubled.find((row) => row.key === 'playlists');

    expect(group && idsOf(doubled, group)).toEqual(['blue']);
  });

  it('a row with nothing below it stands for no track', () => {
    const empty: RowView = { kind: 'playlist', depth: 1, key: 'playlist\u001fEmpty', label: 'Empty' };

    expect(idsOf([empty], empty)).toEqual([]);
  });
});

describe('markStateOf', () => {
  const rows = sampleRows();
  const miles = rows.find((row) => row.key === milesKey);
  if (!miles) {
    throw new Error('the sample has no Miles Davis');
  }

  it('is none, some or all of the tracks a row stands for', () => {
    expect(markStateOf(rows, miles, new Set())).toBe('none');
    expect(markStateOf(rows, miles, new Set(['sowhat']))).toBe('some');
    expect(markStateOf(rows, miles, new Set(['sowhat', 'blue']))).toBe('all');
  });

  it('ignores marks on other tracks', () => {
    expect(markStateOf(rows, miles, new Set(['giant']))).toBe('none');
  });

  it('is none for a row that stands for no track', () => {
    const empty: RowView = { kind: 'playlist', depth: 1, key: 'playlist\u001fEmpty', label: 'Empty' };

    expect(markStateOf([empty], empty, new Set(['x']))).toBe('none');
  });
});

describe('keysToOpenOneLevel', () => {
  it('opens the closed rows at the shallowest closed depth', () => {
    expect(keysToOpenOneLevel(view())).toEqual(['playlists', 'needs', `artist${sep}فيروز`, `artist${sep}John Coltrane`, milesKey, `artist${sep}Nina Simone`]);
  });

  it('goes one level deeper once the top is open', () => {
    const top = keysToOpenOneLevel(view());

    expect(keysToOpenOneLevel(view({ expanded: top }))).toEqual([
      lateNightKey,
      `album${sep}فيروز${sep}Songs`,
      `album${sep}John Coltrane${sep}Giant Steps`,
      kindOfBlueKey,
      `singles${sep}Nina Simone`,
    ]);
  });

  it('leaves the closed rows below an open row alone until the level above is open', () => {
    expect(keysToOpenOneLevel(view({ expanded: [milesKey] }))).toEqual([
      'playlists',
      'needs',
      `artist${sep}فيروز`,
      `artist${sep}John Coltrane`,
      `artist${sep}Nina Simone`,
    ]);
  });

  it('opens nothing when everything is open, while filtering and in a sorted list', () => {
    const everything = sampleRows()
      .filter((row) => row.kind !== 'track')
      .map((row) => row.key);

    expect(keysToOpenOneLevel(view({ expanded: everything }))).toEqual([]);
    expect(keysToOpenOneLevel(view({ filter: 'miles' }))).toEqual([]);
    expect(keysToOpenOneLevel(view({ sortKey: 'title' }))).toEqual([]);
  });
});

describe('settledCursor', () => {
  it('keeps a cursor that is on a visible row', () => {
    expect(settledCursor({ ...view(), cursor: milesKey })).toBe(milesKey);
  });

  it('moves a cursor that is hidden inside a closed row up to the closed row', () => {
    expect(settledCursor({ ...view({ expanded: [milesKey] }), cursor: `track${sep}sowhat` })).toBe(kindOfBlueKey);
    expect(settledCursor({ ...view(), cursor: `track${sep}sowhat` })).toBe(milesKey);
  });

  it('puts a cursor on a row that is gone on the first row', () => {
    expect(settledCursor({ ...view(), cursor: 'artist\u001fGone' })).toBe('playlists');
    expect(settledCursor({ ...view(), cursor: null })).toBe('playlists');
  });

  it('is null when there are no rows', () => {
    expect(settledCursor({ lib: null, filter: '', expanded: new Set(), cursor: 'x' })).toBeNull();
  });
});

describe('ancestorKeys', () => {
  it('lists the rows above a row, the nearest first', () => {
    const rows = sampleRows();
    const sowhat = rows.find((row) => row.key === `track${sep}sowhat`);

    expect(sowhat && ancestorKeys(rows, sowhat)).toEqual([kindOfBlueKey, milesKey]);
  });
});
