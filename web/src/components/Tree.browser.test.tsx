import type { ReactNode } from 'react';
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { page, userEvent } from 'vitest/browser';
import { render } from 'vitest-browser-react';
import type { LibraryResponse } from '../api/types.gen';
import { installKeys } from '../keys/dispatch';
import { loadLibrary } from '../state/library';
import { run } from '../state/actions';
import { useApp } from '../state/store';
import { FakeEventSource, infoView, sampleLibrary, stubServer } from '../testing/fakes';
import '../styles/tokens.css';
import '../styles/app.css';
import { Footer } from './Footer';
import { Tree } from './Tree';

const sep = '\u001f';
const milesKey = `artist${sep}Miles Davis`;
const kindOfBlueKey = `album${sep}Miles Davis${sep}Kind of Blue`;

let removeKeys: () => void;

function Sized({ height, children }: { height: number; children: ReactNode }) {
  return <div style={{ display: 'flex', flexDirection: 'column', width: 380, height }}>{children}</div>;
}

async function showTree(library: LibraryResponse = sampleLibrary(), options: { withFooter?: boolean; height?: number } = {}) {
  stubServer(infoView(), library);
  await loadLibrary();
  return render(
    <Sized height={options.height ?? 280}>
      <Tree />
      {options.withFooter === true && <Footer />}
    </Sized>,
  );
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

function filterBox() {
  return page.getByRole('textbox', { name: 'Filter' });
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

describe('the mouse', () => {

  test('a click in the tree gives it the keys back from the fields', async () => {
    await showTree();
    useApp.setState({ focus: 'inspector' });

    await userEvent.click(page.getByRole('treeitem', { name: /Nina Simone/ }));

    await expect.poll(() => useApp.getState().focus).toBe('tree');
  });
});

describe('the filter', () => {

  test('slash selects what was in the box so that typing replaces it', async () => {
    await showTree();
    await userEvent.fill(filterBox(), 'miles');
    await userEvent.keyboard('{Enter}');

    await userEvent.keyboard('/');
    await userEvent.keyboard('nina');

    await expect.element(filterBox()).toHaveValue('nina');
    await expect.poll(labels).toContain('Feeling Good');
  });

});

describe('giving the keys back to the tree', () => {
  test('enter in the filter box gives the keys to the tree even when they were in the fields', async () => {
    await showTree();
    useApp.setState({ focus: 'inspector' });
    await userEvent.click(filterBox());
    expect(useApp.getState().filterFocused).toBe(true);
    expect(useApp.getState().focus).toBe('inspector');

    await userEvent.keyboard('{Enter}');

    await expect.poll(() => useApp.getState().filterFocused).toBe(false);
    expect(useApp.getState().focus).toBe('tree');
    await userEvent.keyboard('j');
    await expect.poll(cursorLabel).toBe('Needs work');
  });

  test('a click on the empty part of the tree after leaving the filter box gives it the keys', async () => {
    await showTree();
    useApp.setState({ focus: 'inspector' });
    await userEvent.click(filterBox());
    expect(useApp.getState().filterFocused).toBe(true);

    await page.getByRole('tree').click({ position: { x: 20, y: 200 } });

    await expect.poll(() => useApp.getState().filterFocused).toBe(false);
    expect(useApp.getState().focus).toBe('tree');
    await userEvent.keyboard('j');
    await expect.poll(cursorLabel).toBe('Needs work');
  });
});

describe('what a screen reader is told', () => {
  test('a marked row is checked, a part-marked folder is mixed, and an unmarked row says nothing', async () => {
    useApp.setState({ expanded: new Set([milesKey, kindOfBlueKey]) });
    await showTree();
    await userEvent.click(page.getByRole('treeitem', { name: /So What/ }));
    await userEvent.keyboard(' ');

    await expect.poll(() => rowOf('So What').getAttribute('aria-checked')).toBe('true');
    expect(rowOf('Kind of Blue').getAttribute('aria-checked')).toBe('mixed');
    expect(rowOf('Miles Davis').getAttribute('aria-checked')).toBe('mixed');
    expect(rowOf('Blue in Green').hasAttribute('aria-checked')).toBe(false);
  });

  test('the tree names the row under the cursor as its active descendant', async () => {
    useApp.setState({ expanded: new Set([milesKey, kindOfBlueKey]) });
    await showTree();
    await userEvent.click(page.getByRole('treeitem', { name: /Blue in Green/ }));

    await expect.poll(cursorLabel).toBe('Blue in Green');
    const active = page.getByRole('tree').element().getAttribute('aria-activedescendant');
    expect(active).not.toBeNull();
    expect(document.getElementById(active ?? '')).toBe(rowOf('Blue in Green'));
    await userEvent.keyboard('k');
    await expect.poll(() => document.getElementById(page.getByRole('tree').element().getAttribute('aria-activedescendant') ?? '')).toBe(rowOf('So What'));
  });

  test('loading and an empty library are a status, a failure is an alert, and both are laid out in their own direction', async () => {
    useApp.setState({ lib: null });
    await render(<Tree />);
    const loading = page.getByText('Loading…');
    await expect.element(loading).toBeVisible();
    expect(loading.element().closest('[role="status"]')).not.toBeNull();

    useApp.setState({ status: { text: 'تعذر الوصول', kind: 'bad' } });
    const failure = page.getByText('تعذر الوصول');
    await expect.element(failure).toBeVisible();
    expect(failure.element().tagName).toBe('BDI');
    expect(failure.element().getAttribute('dir')).toBe('auto');
    expect(failure.element().closest('[role="alert"]')).not.toBeNull();
  });
});
