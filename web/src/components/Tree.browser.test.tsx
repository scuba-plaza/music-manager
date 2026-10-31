import type { ReactNode } from 'react';
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { page, userEvent } from 'vitest/browser';
import { render } from 'vitest-browser-react';
import type { LibraryResponse } from '../api/types.gen';
import { installKeys } from '../keys/dispatch';
import { loadLibrary, run } from '../state/actions';
import { useApp } from '../state/store';
import { FakeEventSource, infoView, libraryResponse, sampleLibrary, stubServer } from '../testing/fakes';
import '../styles/tokens.css';
import '../styles/app.css';
import { Footer } from './Footer';
import { Tree } from './Tree';

const sep = '\u001f';
const milesKey = `artist${sep}Miles Davis`;
const kindOfBlueKey = `album${sep}Miles Davis${sep}Kind of Blue`;
const topLevel = ['Playlists', 'Needs work', 'فيروز', 'John Coltrane', 'Miles Davis', 'Nina Simone'];

let fetchMock: ReturnType<typeof stubServer>;
let removeKeys: () => void;

function Sized({ height, children }: { height: number; children: ReactNode }) {
  return <div style={{ display: 'flex', flexDirection: 'column', width: 380, height }}>{children}</div>;
}

async function showTree(library: LibraryResponse = sampleLibrary(), options: { withFooter?: boolean; height?: number } = {}) {
  fetchMock = stubServer(infoView(), library);
  await loadLibrary();
  return render(
    <Sized height={options.height ?? 280}>
      <Tree />
      {options.withFooter === true && <Footer />}
    </Sized>,
  );
}

function openEverything(): void {
  useApp.setState({ expanded: new Set(sampleLibrary().view.rows.filter((row) => row.kind !== 'track').map((row) => row.key)) });
}

function rows(): Element[] {
  return page.getByRole('treeitem').elements();
}

function labelOf(row: Element): string {
  return row.querySelector('.label')?.textContent.trim() ?? '';
}

function labels(): string[] {
  return rows().map(labelOf);
}

function cursorLabel(): string | undefined {
  const row = rows().find((candidate) => candidate.getAttribute('aria-selected') === 'true');
  return row && labelOf(row);
}

function rowOf(label: string, nth = 0): Element {
  const row = rows().filter((candidate) => labelOf(candidate) === label)[nth];
  if (!row) {
    throw new Error(`no row ${label}`);
  }
  return row;
}

function markOf(label: string, nth = 0): string {
  return rowOf(label, nth).querySelector('.mark')?.textContent.trim() ?? '';
}

function filterBox() {
  return page.getByRole('textbox', { name: 'Filter' });
}

function libraryRequests(): string[] {
  return fetchMock.mock.calls.flatMap(([input]) => {
    const url = typeof input === 'string' ? input : '';
    return url.startsWith('/api/library') ? [url] : [];
  });
}

function setNativeValue(input: HTMLInputElement, value: string): void {
  Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set?.call(input, value);
}

function compose(input: HTMLInputElement, text: string): void {
  input.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true }));
  setNativeValue(input, text);
  input.dispatchEvent(new InputEvent('input', { bubbles: true, isComposing: true, data: text }));
}

function pause(ms: number): Promise<void> {
  return new Promise((resolve) => {
    setTimeout(resolve, ms);
  });
}

beforeEach(() => {
  localStorage.clear();
  useApp.setState(useApp.getInitialState(), true);
  FakeEventSource.instances = [];
  removeKeys = installKeys(document, run);
});

afterEach(async () => {
  await pause(useApp.getState().filter === '' ? 60 : 200);
  removeKeys();
  vi.unstubAllGlobals();
});

describe('the rows', () => {
  test('the top level lists the groups and the artists, in the order the server sent them', async () => {
    await showTree();

    await expect.poll(labels).toEqual(topLevel);
    await expect.element(page.getByRole('tree')).toBeVisible();
  });

  test('the first row has the cursor, and only one row has it', async () => {
    await showTree();

    await expect.poll(cursorLabel).toBe('Playlists');
    expect(rows().filter((row) => row.getAttribute('aria-selected') === 'true')).toHaveLength(1);
    expect(rows().filter((row) => row.classList.contains('cursor'))).toHaveLength(1);
  });

  test('a row says how deep it is and whether it is open', async () => {
    await showTree();
    await userEvent.keyboard('{ArrowDown}{ArrowDown}{ArrowDown}{ArrowDown}{Enter}');

    await expect.poll(labels).toContain('Kind of Blue');
    expect(rowOf('Miles Davis').getAttribute('aria-level')).toBe('1');
    expect(rowOf('Miles Davis').getAttribute('aria-expanded')).toBe('true');
    expect(rowOf('Kind of Blue').getAttribute('aria-level')).toBe('2');
    expect(rowOf('Kind of Blue').getAttribute('aria-expanded')).toBe('false');
    expect(rowOf('Nina Simone').getAttribute('aria-expanded')).toBe('false');
  });

  test('a track has no aria-expanded and is indented below its album', async () => {
    useApp.setState({ expanded: new Set([milesKey, kindOfBlueKey]) });
    await showTree();

    await expect.poll(labels).toContain('So What');
    const track = rowOf('So What');
    expect(track.getAttribute('aria-level')).toBe('3');
    expect(track.hasAttribute('aria-expanded')).toBe(false);
    const indent = (row: Element) => row.querySelector('.label')?.getBoundingClientRect().left ?? 0;
    expect(indent(track)).toBeGreaterThan(indent(rowOf('Kind of Blue')));
    expect(indent(rowOf('Kind of Blue'))).toBeGreaterThan(indent(rowOf('Miles Davis')));
  });

  test('a track row shows its number, its title, who it is by and its length; a folder its tracks', async () => {
    useApp.setState({ expanded: new Set(['needs', milesKey, kindOfBlueKey]) });
    await showTree();

    await expect.poll(labels).toContain('Rough Mix');
    const rough = rowOf('Rough Mix');
    expect(rough.querySelector('.detail')?.textContent.trim()).toBe('Basement Tapes');
    expect(rough.querySelector('.right')?.textContent.trim()).toBe('0:05');
    expect(rowOf('So What').querySelector('.num')?.textContent.trim()).toBe('01');
    expect(rowOf('Blue in Green').querySelector('.num')?.textContent.trim()).toBe('03');
    expect(rowOf('Miles Davis').querySelector('.right')?.textContent.trim()).toBe('2');
    expect(rowOf('Needs work').querySelector('.right')?.textContent.trim()).toBe('1');
  });

  test('times are set in the monospace face and names are not', async () => {
    useApp.setState({ expanded: new Set(['needs']) });
    await showTree();

    await expect.poll(labels).toContain('Rough Mix');
    const rough = rowOf('Rough Mix');
    const face = (element: Element | null) => (element ? getComputedStyle(element).fontFamily : '');
    expect(face(rough.querySelector('.right'))).toMatch(/monospace$/);
    expect(face(rough.querySelector('.label'))).not.toMatch(/monospace/);
    expect(face(rough.querySelector('.detail'))).not.toMatch(/monospace/);
  });

  test('every name is a bdi with dir=auto, the Arabic ones included', async () => {
    useApp.setState({ expanded: new Set(['needs', `artist${sep}فيروز`]) });
    await showTree();

    await expect.poll(labels).toContain('Rough Mix');
    const names = rows().flatMap((row) => [...row.querySelectorAll('.label > *, .detail > *')]);
    expect(names.length).toBeGreaterThan(topLevel.length);
    for (const name of names) {
      expect(name.tagName).toBe('BDI');
      expect(name.getAttribute('dir')).toBe('auto');
    }
    expect(rowOf('فيروز').querySelector('bdi')?.textContent).toBe('فيروز');
  });

  test('a sorted list is flat: no arrows, no indentation, and enter goes straight to the fields', async () => {
    const sorted = sampleLibrary();
    sorted.view.rows = sorted.view.rows.filter((row) => row.kind === 'track').map((row) => ({ ...row, depth: 0 }));
    sorted.view.sort = { ...sorted.view.sort, key: 'title', title: 'Library · title ↑' };
    await showTree(sorted);

    await expect.poll(() => rows().length).toBe(7);
    for (const row of rows()) {
      expect(row.getAttribute('aria-level')).toBe('1');
      expect(row.hasAttribute('aria-expanded')).toBe(false);
      expect(row.querySelector('.twist')?.textContent).toBe('');
    }
    await userEvent.keyboard('{Enter}');
    expect(useApp.getState().focus).toBe('inspector');
  });

  test('an empty library says so', async () => {
    await showTree(libraryResponse());

    await expect.element(page.getByText('The library is empty.', { exact: false })).toBeVisible();
    expect(rows()).toHaveLength(0);
  });

  test('a filter that matches nothing says so', async () => {
    await showTree();

    await userEvent.fill(filterBox(), 'zzzz');

    await expect.element(page.getByText('Nothing matches the filter.')).toBeVisible();
  });
});

describe('the cursor', () => {
  test('moves the cursor with j and k and the arrow keys', async () => {
    await showTree();
    await expect.poll(cursorLabel).toBe('Playlists');

    await userEvent.keyboard('j');
    await expect.poll(cursorLabel).toBe('Needs work');

    await userEvent.keyboard('{ArrowDown}');
    await expect.poll(cursorLabel).toBe('فيروز');

    await userEvent.keyboard('k');
    await expect.poll(cursorLabel).toBe('Needs work');

    await userEvent.keyboard('{ArrowUp}');
    await expect.poll(cursorLabel).toBe('Playlists');

    await userEvent.keyboard('{ArrowUp}');
    expect(cursorLabel()).toBe('Playlists');
  });

  test('Home and g go to the top, End and G to the bottom', async () => {
    await showTree();

    await userEvent.keyboard('{End}');
    await expect.poll(cursorLabel).toBe('Nina Simone');

    await userEvent.keyboard('{Home}');
    await expect.poll(cursorLabel).toBe('Playlists');

    await userEvent.keyboard('G');
    await expect.poll(cursorLabel).toBe('Nina Simone');

    await userEvent.keyboard('g');
    await expect.poll(cursorLabel).toBe('Playlists');
  });

  test('PageDown and PageUp, ctrl+d and ctrl+u move by the rows that fit, less one', async () => {
    openEverything();
    await showTree(sampleLibrary(), { height: 170 });
    await expect.poll(() => rows().length).toBe(18);
    const tree = page.getByRole('tree').element();
    const height = rows()[0]?.getBoundingClientRect().height ?? 1;
    const page1 = Math.floor(tree.clientHeight / height) - 1;
    expect(page1).toBeGreaterThan(2);
    expect(page1 * 2).toBeLessThan(rows().length);
    const index = () => rows().findIndex((row) => row.getAttribute('aria-selected') === 'true');

    await userEvent.keyboard('{PageDown}');
    await expect.poll(index).toBe(page1);

    await userEvent.keyboard('{Control>}d{/Control}');
    await expect.poll(index).toBe(page1 * 2);

    await userEvent.keyboard('{PageUp}');
    await expect.poll(index).toBe(page1);

    await userEvent.keyboard('{Control>}u{/Control}');
    await expect.poll(index).toBe(0);
  });

  test('the row under the cursor is scrolled into view', async () => {
    openEverything();
    await showTree();
    await expect.poll(() => rows().length).toBe(18);
    const tree = page.getByRole('tree').element();
    expect(tree.scrollHeight).toBeGreaterThan(tree.clientHeight);

    await userEvent.keyboard('{End}');

    await expect.poll(cursorLabel).toBe('Feeling Good');
    await expect
      .poll(() => {
        const box = tree.getBoundingClientRect();
        const row = rowOf('Feeling Good').getBoundingClientRect();
        return row.top >= box.top - 1 && row.bottom <= box.bottom + 1;
      })
      .toBe(true);

    await userEvent.keyboard('{Home}');

    await expect
      .poll(() => {
        const box = tree.getBoundingClientRect();
        const row = rowOf('Playlists').getBoundingClientRect();
        return row.top >= box.top - 1 && row.bottom <= box.bottom + 1;
      })
      .toBe(true);
  });

  test('z puts the cursor row in the middle of the tree', async () => {
    openEverything();
    await showTree();
    await expect.poll(() => rows().length).toBe(18);
    const tree = page.getByRole('tree').element();
    const rowHeight = rows()[0]?.getBoundingClientRect().height ?? 1;
    await userEvent.keyboard('{End}');
    await userEvent.keyboard('kkkkkkkkk');
    await expect.poll(cursorLabel).toBe('John Coltrane');

    await userEvent.keyboard('z');

    await expect
      .poll(() => {
        const box = tree.getBoundingClientRect();
        const row = rowOf('John Coltrane').getBoundingClientRect();
        return Math.abs(row.top + row.height / 2 - (box.top + box.height / 2)) <= rowHeight;
      })
      .toBe(true);
  });

  test('typing in the filter box does not move the cursor, open anything or mark', async () => {
    await showTree();
    await userEvent.click(filterBox());

    await userEvent.keyboard('jk l+- Gz');

    await expect.element(filterBox()).toHaveValue('jk l+- Gz');
    expect(cursorLabel()).toBe('Playlists');
    expect(useApp.getState().marks.size).toBe(0);
    expect(useApp.getState().expanded.size).toBe(0);
  });

  test('the cursor row shows its details once it rests', async () => {
    await showTree();

    await userEvent.keyboard('j');

    await expect.poll(() => useApp.getState().detail).toMatchObject({ label: 'Needs work' });
  });
});

describe('opening and closing', () => {
  test('enter opens an artist and shows its albums', async () => {
    await showTree();
    await userEvent.keyboard('{ArrowDown}{ArrowDown}{ArrowDown}{ArrowDown}');
    await expect.poll(cursorLabel).toBe('Miles Davis');
    expect(labels()).not.toContain('Kind of Blue');

    await userEvent.keyboard('{Enter}');

    await expect.poll(labels).toEqual([...topLevel.slice(0, 5), 'Kind of Blue', 'Nina Simone']);
    expect(cursorLabel()).toBe('Miles Davis');

    await userEvent.keyboard('{Enter}');

    await expect.poll(labels).toEqual(topLevel);
  });

  test('l opens a row and steps into it, h closes it or goes up to its parent', async () => {
    await showTree();

    await userEvent.keyboard('l');
    await expect.poll(labels).toContain('Late night');
    expect(cursorLabel()).toBe('Playlists');

    await userEvent.keyboard('{ArrowRight}');
    await expect.poll(cursorLabel).toBe('Late night');

    await userEvent.keyboard('h');
    await expect.poll(cursorLabel).toBe('Playlists');

    await userEvent.keyboard('{ArrowLeft}');
    await expect.poll(labels).toEqual(topLevel);
    expect(cursorLabel()).toBe('Playlists');
  });

  test('+ and = open one level more, - closes everything', async () => {
    await showTree();

    await userEvent.keyboard('+');
    await expect.poll(() => rows().length).toBe(12);
    expect(labels()).toContain('Songs');
    expect(labels()).not.toContain('وطني');

    await userEvent.keyboard('=');
    await expect.poll(labels).toContain('وطني');
    expect(labels()).toContain('Feeling Good');
    expect(rows()).toHaveLength(18);

    await userEvent.keyboard('-');
    await expect.poll(labels).toEqual(topLevel);
  });

  test('closing everything keeps the cursor on the row that holds it', async () => {
    useApp.setState({ expanded: new Set([milesKey, kindOfBlueKey]) });
    await showTree();
    await userEvent.keyboard('{End}');
    await userEvent.keyboard('kk');
    await expect.poll(cursorLabel).toBe('So What');

    await userEvent.keyboard('-');

    await expect.poll(cursorLabel).toBe('Miles Davis');
  });

  test('the rows that are open are remembered', async () => {
    await showTree();

    await userEvent.keyboard('{ArrowDown}{ArrowDown}{ArrowDown}{ArrowDown}{Enter}');

    await expect.poll(labels).toContain('Kind of Blue');
    expect(JSON.parse(localStorage.getItem('mm.expanded') ?? '[]')).toEqual([milesKey]);
  });
});

describe('the mouse', () => {
  test('a click puts the cursor on a row and a double click opens it', async () => {
    await showTree();

    await userEvent.click(page.getByRole('treeitem', { name: /Miles Davis/ }));

    await expect.poll(cursorLabel).toBe('Miles Davis');
    expect(labels()).not.toContain('Kind of Blue');

    await userEvent.dblClick(page.getByRole('treeitem', { name: /Miles Davis/ }));

    await expect.poll(labels).toContain('Kind of Blue');
    expect(cursorLabel()).toBe('Miles Davis');

    await userEvent.dblClick(page.getByRole('treeitem', { name: /Miles Davis/ }));

    await expect.poll(labels).not.toContain('Kind of Blue');
  });

  test('a double click on a track goes into its fields', async () => {
    useApp.setState({ expanded: new Set(['needs']) });
    await showTree();

    await userEvent.dblClick(page.getByRole('treeitem', { name: /Rough Mix/ }));

    await expect.poll(() => useApp.getState().focus).toBe('inspector');
    expect(cursorLabel()).toBe('Rough Mix');
  });

  test('a click on the arrow opens or closes a row at once and puts the cursor on it', async () => {
    await showTree();
    const twist = () => rowOf('Miles Davis').querySelector('.twist');
    expect(twist()?.textContent).toBe('▸');

    await userEvent.click(twist() ?? document.body);

    await expect.poll(labels).toContain('Kind of Blue');
    expect(cursorLabel()).toBe('Miles Davis');
    expect(twist()?.textContent).toBe('▾');

    await userEvent.click(twist() ?? document.body);

    await expect.poll(labels).not.toContain('Kind of Blue');
  });

  test('a click on the marker marks the row', async () => {
    useApp.setState({ expanded: new Set([milesKey, kindOfBlueKey]) });
    await showTree();

    await userEvent.click(rowOf('So What').querySelector('.mark') ?? document.body);

    await expect.poll(() => markOf('So What')).toBe('●');
    expect([...useApp.getState().marks]).toEqual(['sowhat']);
  });

  test('a click in the tree gives it the keys back from the fields', async () => {
    await showTree();
    useApp.setState({ focus: 'inspector' });

    await userEvent.click(page.getByRole('treeitem', { name: /Nina Simone/ }));

    await expect.poll(() => useApp.getState().focus).toBe('tree');
  });
});

describe('marks', () => {
  test('space marks and an album marks all its tracks', async () => {
    useApp.setState({ expanded: new Set([milesKey, kindOfBlueKey]) });
    await showTree();
    await userEvent.click(page.getByRole('treeitem', { name: /Kind of Blue/ }));
    await expect.poll(cursorLabel).toBe('Kind of Blue');
    expect(markOf('Kind of Blue')).toBe('');

    await userEvent.keyboard(' ');

    await expect.poll(() => markOf('Kind of Blue')).toBe('●');
    expect(markOf('So What')).toBe('●');
    expect(markOf('Blue in Green')).toBe('●');
    expect(markOf('Miles Davis')).toBe('●');
    expect([...useApp.getState().marks].toSorted()).toEqual(['blue', 'sowhat']);
    expect(cursorLabel()).toBe('So What');
    expect(rowOf('So What').getAttribute('data-mark')).toBe('all');

    await userEvent.keyboard(' ');

    await expect.poll(() => markOf('So What')).toBe('');
    expect(markOf('Kind of Blue')).toBe('◐');
    expect(markOf('Blue in Green')).toBe('●');
    expect(cursorLabel()).toBe('Blue in Green');
  });

  test('space on a marked album takes the marks away again', async () => {
    useApp.setState({ expanded: new Set([milesKey]) });
    await showTree();
    await userEvent.click(page.getByRole('treeitem', { name: /Kind of Blue/ }));

    await userEvent.keyboard(' ');
    await expect.poll(() => markOf('Kind of Blue')).toBe('●');
    await userEvent.click(page.getByRole('treeitem', { name: /Kind of Blue/ }));
    await userEvent.keyboard(' ');

    await expect.poll(() => markOf('Kind of Blue')).toBe('');
    expect(useApp.getState().marks.size).toBe(0);
  });

  test('ctrl+a marks every track shown, and again takes the marks away', async () => {
    useApp.setState({ expanded: new Set([milesKey, kindOfBlueKey, 'needs']) });
    await showTree();

    await userEvent.keyboard('{Control>}a{/Control}');

    await expect.poll(() => [...useApp.getState().marks].toSorted()).toEqual(['blue', 'rough', 'sowhat']);
    expect(markOf('Rough Mix')).toBe('●');
    expect(markOf('John Coltrane')).toBe('');

    await userEvent.keyboard('{Control>}a{/Control}');

    await expect.poll(() => useApp.getState().marks.size).toBe(0);
  });

  test('escape clears the marks when there is no filter', async () => {
    useApp.setState({ expanded: new Set([milesKey]), marks: new Set(['blue']) });
    await showTree();

    await userEvent.keyboard('{Escape}');

    await expect.poll(() => useApp.getState().marks.size).toBe(0);
  });
});

describe('the filter', () => {
  test('the filter box narrows the tree through the server', async () => {
    await showTree();
    const asked = libraryRequests().length;

    await userEvent.fill(filterBox(), 'miles');

    await expect
      .poll(labels)
      .toEqual(['Playlists', 'Late night', 'Blue in Green', 'Miles Davis', 'Kind of Blue', 'So What', 'Blue in Green']);
    expect(libraryRequests().slice(asked)).toEqual(['/api/library?q=miles']);
    expect(useApp.getState().filter).toBe('miles');
    expect(rowOf('Miles Davis').getAttribute('aria-expanded')).toBe('true');
    expect(rowOf('Miles Davis').querySelector('.twist')?.textContent).toBe('');
  });

  test('the server words reach the server as they were typed', async () => {
    await showTree();

    await userEvent.fill(filterBox(), 'status:unidentified');

    await expect.poll(labels).toEqual(['Needs work', 'Rough Mix']);
    expect(libraryRequests().at(-1)).toBe('/api/library?q=status%3Aunidentified');
  });

  test('a burst of typing is one request', async () => {
    await showTree();
    const asked = libraryRequests().length;

    await userEvent.type(filterBox(), 'miles');

    await expect.poll(labels).toContain('Kind of Blue');
    await pause(300);
    expect(libraryRequests().slice(asked)).toEqual(['/api/library?q=miles']);
  });

  test('escape clears the filter', async () => {
    await showTree();
    await userEvent.fill(filterBox(), 'miles');
    await expect.poll(() => rows().length).toBe(7);

    await userEvent.keyboard('{Escape}');

    await expect.element(filterBox()).toHaveValue('');
    await expect.poll(labels).toEqual(topLevel);
    expect(useApp.getState().filter).toBe('');
    expect(document.activeElement).not.toBe(filterBox().element());
    expect(libraryRequests().at(-1)).toBe('/api/library');
  });

  test('escape in the tree clears a filter that was kept with enter', async () => {
    await showTree();
    await userEvent.fill(filterBox(), 'miles');
    await expect.poll(() => rows().length).toBe(7);
    await userEvent.keyboard('{Enter}');
    expect(useApp.getState().filter).toBe('miles');

    await userEvent.keyboard('{Escape}');

    await expect.element(filterBox()).toHaveValue('');
    await expect.poll(labels).toEqual(topLevel);
  });

  test('slash focuses the filter box; enter and the down arrow hand the keys back to the tree', async () => {
    await showTree();

    await userEvent.keyboard('/');

    await expect.element(filterBox()).toHaveFocus();
    await userEvent.keyboard('miles');
    await expect.poll(() => rows().length).toBe(7);

    await userEvent.keyboard('{Enter}');

    await expect.element(page.getByRole('tree')).toHaveFocus();
    await expect.element(filterBox()).toHaveValue('miles');
    expect(labels()).toContain('Kind of Blue');

    await userEvent.keyboard('/');
    await expect.element(filterBox()).toHaveFocus();
    await userEvent.keyboard('{ArrowDown}');

    await expect.element(page.getByRole('tree')).toHaveFocus();
  });

  test('slash selects what was in the box so that typing replaces it', async () => {
    await showTree();
    await userEvent.fill(filterBox(), 'miles');
    await userEvent.keyboard('{Enter}');

    await userEvent.keyboard('/');
    await userEvent.keyboard('nina');

    await expect.element(filterBox()).toHaveValue('nina');
    await expect.poll(labels).toContain('Feeling Good');
  });

  test('nothing is asked while an input method is composing, and the text is asked for once it ends', async () => {
    await showTree();
    const box = filterBox().element() as HTMLInputElement;
    const asked = libraryRequests().length;

    box.focus();
    compose(box, 'm');
    compose(box, 'mi');
    await pause(400);

    expect(libraryRequests().slice(asked)).toEqual([]);
    expect(useApp.getState().filter).toBe('mi');
    await expect.element(filterBox()).toHaveValue('mi');

    box.dispatchEvent(new CompositionEvent('compositionend', { bubbles: true, data: 'mi' }));

    await expect.poll(() => libraryRequests().slice(asked)).toEqual(['/api/library?q=mi']);
  });

  test('the footer shows the keys of the filter box while it has the focus', async () => {
    await showTree(sampleLibrary(), { withFooter: true });
    const hints = () =>
      page
        .getByRole('list', { name: 'Keys' })
        .getByRole('listitem')
        .elements()
        .map((item) => item.textContent.trim());
    await expect.poll(hints).toContain('/ filter');

    await userEvent.keyboard('/');

    await expect.poll(hints).toEqual(['enter keep', 'esc clear']);

    await userEvent.keyboard('{Enter}');

    await expect.poll(hints).toContain('/ filter');
    expect(hints()).not.toContain('enter keep');

    await userEvent.click(filterBox());
    await expect.poll(hints).toEqual(['enter keep', 'esc clear']);

    await userEvent.click(page.getByRole('treeitem', { name: /Nina Simone/ }));
    await expect.poll(hints).toContain('/ filter');
  });
});
