import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { PeekView, TrackView } from '../api/types.gen';
import { fieldView, infoView, memoryStorage, sampleLibrary, stubServer } from '../testing/fakes';
import { run } from './actions';
import { currentField, fieldIndex, tabIndex } from './inspector';
import { useApp } from './store';

vi.mock('./treeDom', () => ({
  pageSize: vi.fn(() => 3),
  centreCursor: vi.fn(),
  focusFilterBox: vi.fn(),
  leaveFilterBox: vi.fn(),
}));

const sep = '\u001f';
const keys = {
  playlists: 'playlists',
  lateNight: `playlist${sep}Late night`,
  singles: `singles${sep}Nina Simone`,
  miles: `artist${sep}Miles Davis`,
  soWhat: `track${sep}sowhat`,
};

function trackView(): TrackView {
  return {
    id: 'sowhat',
    title: 'So What',
    identified: true,
    tabs: [
      { id: 'details', name: 'Details', fields: [fieldView('title'), fieldView('artist'), fieldView('album')] },
      { id: 'audio', name: 'Audio', fields: [fieldView('cutStart'), fieldView('cutEnd')] },
      { id: 'latin', name: 'Latin', fields: [fieldView('latinTitle')] },
      { id: 'device', name: 'Device', fields: [] },
    ],
    path: '',
    dates: '',
    about: '',
    reveal: [],
  };
}

function artistView(): PeekView {
  return { kind: 'artist', title: 'Artist', label: 'Miles Davis', editable: true, info: [], fields: [fieldView('artist'), fieldView('genre')], list: [] };
}

beforeEach(() => {
  vi.stubGlobal('localStorage', memoryStorage());
  useApp.setState(useApp.getInitialState(), true);
  stubServer(infoView(), sampleLibrary());
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function onTrack(extra: Partial<ReturnType<typeof useApp.getState>> = {}): void {
  useApp.setState({ lib: sampleLibrary(), cursor: keys.soWhat, detail: trackView(), focus: 'inspector', ...extra });
}

describe('what the fields are', () => {
  it('the tab shown is kept within the tabs the track has', () => {
    expect(tabIndex(trackView(), 2)).toBe(2);
    expect(tabIndex(trackView(), 9)).toBe(3);
  });

  it('the field cursor is kept within the fields of the tab shown', () => {
    onTrack({ tab: 1, field: 5 });

    expect(fieldIndex(useApp.getState())).toBe(1);
    expect(currentField(useApp.getState())?.key).toBe('cutEnd');
  });

  it('there is no current field while the keyboard is in the tree', () => {
    onTrack({ focus: 'tree' });

    expect(currentField(useApp.getState())).toBeUndefined();
  });

  it('an artist has its own fields and no tabs', () => {
    useApp.setState({ lib: sampleLibrary(), cursor: keys.miles, detail: artistView(), focus: 'inspector', tab: 2, field: 1 });

    expect(currentField(useApp.getState())?.key).toBe('genre');
  });
});

describe('choosing a field', () => {
  it('fieldDown and fieldUp stop at the last and the first field', () => {
    onTrack();

    run('fieldDown');
    run('fieldDown');
    run('fieldDown');
    expect(useApp.getState().field).toBe(2);

    run('fieldUp');
    run('fieldUp');
    run('fieldUp');
    expect(useApp.getState().field).toBe(0);
  });

  it('fieldDown starts from the field shown when the cursor was beyond the fields', () => {
    onTrack({ tab: 1, field: 6 });

    run('fieldUp');

    expect(useApp.getState().field).toBe(0);
  });
});

describe('choosing a tab', () => {
  it('nextTab turns to the next tab, from the last back to the first, and starts at the first field', () => {
    onTrack({ field: 2 });

    run('nextTab');
    expect(useApp.getState()).toMatchObject({ tab: 1, field: 0 });

    run('nextTab');
    run('nextTab');
    run('nextTab');
    expect(useApp.getState().tab).toBe(0);
  });

  it('previousTabOrBack turns back a tab, and on the first goes back to the tree', () => {
    onTrack({ tab: 2 });

    run('previousTabOrBack');
    expect(useApp.getState()).toMatchObject({ tab: 1, focus: 'inspector' });

    run('previousTabOrBack');
    expect(useApp.getState()).toMatchObject({ tab: 0, focus: 'inspector' });

    run('previousTabOrBack');
    expect(useApp.getState()).toMatchObject({ tab: 0, focus: 'tree' });
  });

  it('the number keys choose a tab by its number, and a number beyond the tabs does nothing', () => {
    onTrack({ field: 1 });

    run('tab3');
    expect(useApp.getState()).toMatchObject({ tab: 2, field: 0 });

    run('tab4');
    expect(useApp.getState().tab).toBe(3);

    useApp.setState({ detail: { ...trackView(), tabs: trackView().tabs.slice(0, 3) }, tab: 0 });
    run('tab4');
    expect(useApp.getState().tab).toBe(0);
  });

  it('turning to another tab closes a field that was being typed in', () => {
    onTrack({ edit: { field: 1, text: 'draft' } });

    run('tab2');

    expect(useApp.getState().edit).toBeNull();
  });

  it('the tab is remembered', () => {
    onTrack();

    run('tab2');

    expect(localStorage.getItem('mm.tab')).toBe('1');
  });

  it('an artist has no tabs: the tab keys leave it alone and ← goes back to the tree', () => {
    useApp.setState({ lib: sampleLibrary(), cursor: keys.miles, detail: artistView(), focus: 'inspector', tab: 0 });

    run('nextTab');
    run('tab2');
    expect(useApp.getState().tab).toBe(0);

    run('previousTabOrBack');
    expect(useApp.getState().focus).toBe('tree');
  });
});

describe('the other pane', () => {
  it('Tab goes into the fields of a track and back to the tree', () => {
    useApp.setState({ lib: sampleLibrary(), cursor: keys.soWhat, expanded: new Set([keys.miles, `album${sep}Miles Davis${sep}Kind of Blue`]) });

    run('toggleFocus');
    expect(useApp.getState().focus).toBe('inspector');

    run('toggleFocus');
    expect(useApp.getState().focus).toBe('tree');
  });

  it('Tab goes into the fields of an artist or a playlist at the first field', () => {
    useApp.setState({ lib: sampleLibrary(), cursor: keys.lateNight, field: 3 });

    run('toggleFocus');

    expect(useApp.getState()).toMatchObject({ focus: 'inspector', field: 0 });
  });

  it('Tab stays in the tree on what has no fields: the Playlists group and Singles', () => {
    useApp.setState({ lib: sampleLibrary(), cursor: keys.playlists, expanded: new Set([keys.playlists]) });
    run('toggleFocus');
    expect(useApp.getState().focus).toBe('tree');

    useApp.setState({ cursor: keys.singles, expanded: new Set([`artist${sep}Nina Simone`]) });
    run('toggleFocus');
    expect(useApp.getState().focus).toBe('tree');
  });
});
