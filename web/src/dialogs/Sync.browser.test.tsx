import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { page, userEvent } from 'vitest/browser';
import { render } from 'vitest-browser-react';
import type { JobView, OpCounts, OpView, PlanView, SyncData, TargetView, WarningView } from '../api/types.gen';
import { App } from '../App';
import { goToRow } from '../state/edit';
import { jobEvent } from '../state/jobs';
import { useApp } from '../state/store';
import { useSync } from '../state/sync';
import { FakeEventSource, infoView, jsonResponse, requestsTo, sampleLibrary, sampleProfiles, stubServer, type StubRoute } from '../testing/fakes';

const sep = '\u001f';
const soWhat = `track${sep}sowhat`;

let fetchMock: ReturnType<typeof stubServer>;

const targets: TargetView[] = [
  { label: 'Library folder', path: '~/Music', kind: 'library' },
  { label: 'USB', path: '/media/me/USB', kind: 'drive' },
];

const profiles = [
  { name: 'generic', label: 'Generic', builtin: true },
  { name: 'wolfgang-wmp01', label: 'Wolfgang WMP01', builtin: true },
  { name: 'my-player', label: 'My Player', builtin: false },
];

function counts(over: Partial<OpCounts> = {}): OpCounts {
  return { create: 1, update: 0, stray: 0, remove: 0, ...over };
}

function plan(over: Partial<PlanView> = {}): PlanView {
  return { plan: 'p1', where: '~/player', prune: false, ops: [{ kind: 'create', path: 'Miles Davis/So What.mp3' }], counts: counts(), text: '1 new', warnings: [], ...over };
}

function job(state: string, data?: SyncData, error?: string): JobView {
  return { id: 'j1', kind: 'sync', state, ...(data === undefined ? {} : { data }), ...(error === undefined ? {} : { error }) };
}

function dialog() {
  return page.getByRole('dialog');
}

function hints(): string[] {
  return page
    .getByRole('list', { name: 'Keys' })
    .getByRole('listitem')
    .elements()
    .map((item) => item.textContent.trim());
}

function format() {
  return dialog().getByRole('combobox', { name: 'Format' });
}

function targetList(): string[] {
  return dialog()
    .getByRole('listbox', { name: 'Where to write' })
    .getByRole('option')
    .elements()
    .map((option) => [...option.children].map((cell) => cell.textContent.trim()).join('|'));
}

function chosenTarget(): string | undefined {
  return dialog()
    .getByRole('listbox', { name: 'Where to write' })
    .getByRole('option', { selected: true })
    .element()
    .querySelector('.label')?.textContent.trim();
}

function emit(view: JobView): void {
  FakeEventSource.instances[0]?.emit('job', JSON.stringify(view));
}

const served: StubRoute = ({ method, url }) => {
  if (method === 'GET' && url.pathname === '/api/sync/targets') {
    return jsonResponse({ targets });
  }
  if (method === 'POST' && url.pathname === '/api/sync/plan') {
    return jsonResponse(job('running'));
  }
  if (method === 'POST' && url.pathname === '/api/sync/p1/apply') {
    return jsonResponse(job('running'));
  }
  if (method === 'DELETE' && url.pathname === '/api/jobs/j1') {
    return jsonResponse(job('stopped'));
  }
  return undefined;
};

function routes(...all: StubRoute[]): StubRoute {
  return (request) => {
    for (const route of all) {
      const answer = route(request);
      if (answer !== undefined) {
        return answer;
      }
    }
    return undefined;
  };
}

async function openApp(...extra: StubRoute[]): Promise<void> {
  fetchMock = stubServer(infoView(), sampleLibrary(), { route: routes(...extra, served), profiles: sampleProfiles() });
  await render(<App />);
  await expect.element(page.getByRole('treeitem').first()).toBeVisible();
  useApp.setState({ profiles });
  goToRow(soWhat);
  page.getByRole('tree').element().focus();
  await expect.element(page.getByRole('tree').getByRole('treeitem', { selected: true })).toBeVisible();
}

async function open(...extra: StubRoute[]): Promise<void> {
  await openApp(...extra);
  await userEvent.keyboard('s');
  await expect.element(dialog()).toHaveAccessibleName('Sync');
}

async function typing(...extra: StubRoute[]): Promise<void> {
  await open(...extra);
  await userEvent.keyboard('{End}{Enter}');
  await expect.element(dialog().getByRole('textbox', { name: 'Folder' })).toHaveFocus();
}

async function planning(...extra: StubRoute[]): Promise<void> {
  await open(...extra);
  await userEvent.keyboard('j{Enter}');
  await expect.element(dialog()).toHaveTextContent('Preparing');
  await expect.poll(() => requestsTo(fetchMock, 'POST', '/api/sync/plan').length).toBe(1);
  await expect.poll(() => useSync.getState().job?.id).toBe('j1');
}

async function reviewing(over: Partial<PlanView> = {}, ...extra: StubRoute[]): Promise<void> {
  await planning(...extra);
  emit(job('done', { phase: 'plan', plan: plan(over) }));
  await expect.poll(() => useSync.getState().phase).toBe('reviewing');
  await expect.element(dialog()).toHaveTextContent(/→|Nothing to do/);
}

function ops(n: number, kind = 'create'): OpView[] {
  return Array.from({ length: n }, (_, i) => ({ kind, path: `Artist/Track ${String(i + 1)}.mp3` }));
}

function warnings(n: number): WarningView[] {
  return Array.from({ length: n }, (_, i) => ({ track: `Track ${String(i + 1)}`, message: `message ${String(i + 1)}` }));
}

beforeEach(() => {
  localStorage.clear();
  useApp.setState(useApp.getInitialState(), true);
  useSync.setState(useSync.getInitialState(), true);
  FakeEventSource.instances = [];
});

afterEach(async () => {
  useApp.setState({ overlay: null });
  jobEvent({ id: 'j1', kind: 'sync', state: 'stopped' });
  await new Promise((done) => setTimeout(done, 80));
  vi.unstubAllGlobals();
});

describe('choosing where', () => {
  test('lists the library folder, the drives and another folder, with the paths beside them', async () => {
    await open();

    expect(targetList()).toEqual(['|Library folder|~/Music', '|USB|/media/me/USB', '|Another folder…|']);
    expect(chosenTarget()).toBe('Library folder');
    await expect.element(dialog()).toHaveTextContent('Where should the music be written?');
    await expect.poll(hints).toEqual(['↑↓ choose', 'enter choose', 'esc close']);
  });

  test('the library folder has no format to choose, and says why', async () => {
    await open();

    await expect.element(format()).not.toBeInTheDocument();
    await expect.element(dialog()).toHaveTextContent('Format Generic · the library folder is always written the standard way');
  });

  test('any other place has the formats to choose from, generic chosen, and w is a hint', async () => {
    await open();

    await userEvent.keyboard('j');

    await expect.element(format()).toHaveValue('generic');
    expect([...format().element().querySelectorAll('option')].map((option) => [option.value, option.textContent])).toEqual([
      ['generic', 'Generic'],
      ['wolfgang-wmp01', 'Wolfgang WMP01'],
      ['my-player', 'My Player'],
    ]);
    await expect.element(dialog()).toHaveTextContent('· w goes through them');
    await expect.poll(hints).toEqual(['↑↓ choose', 'enter choose', 'w format', 'esc close']);
  });

  test('the arrows, j and k, home and end move the choice and stop at the ends', async () => {
    await open();

    await userEvent.keyboard('{ArrowUp}');
    expect(chosenTarget()).toBe('Library folder');
    await userEvent.keyboard('j');
    expect(chosenTarget()).toBe('USB');
    await userEvent.keyboard('{ArrowDown}');
    expect(chosenTarget()).toBe('Another folder…');
    await userEvent.keyboard('{ArrowDown}');
    expect(chosenTarget()).toBe('Another folder…');
    await userEvent.keyboard('k');
    expect(chosenTarget()).toBe('USB');
    await userEvent.keyboard('{ArrowUp}');
    expect(chosenTarget()).toBe('Library folder');
    await userEvent.keyboard('{End}');
    expect(chosenTarget()).toBe('Another folder…');
    await userEvent.keyboard('{Home}');
    expect(chosenTarget()).toBe('Library folder');
  });

  test('w goes through the formats, and does nothing for the library folder', async () => {
    await open();
    await userEvent.keyboard('w');
    await expect.element(format()).not.toBeInTheDocument();
    await userEvent.keyboard('j');

    await userEvent.keyboard('w');
    await expect.element(format()).toHaveValue('wolfgang-wmp01');
    await userEvent.keyboard('w');
    await expect.element(format()).toHaveValue('my-player');
    await userEvent.keyboard('w');
    await expect.element(format()).toHaveValue('generic');
  });

  test('choosing a format in the list sets it', async () => {
    await open();
    await userEvent.keyboard('j');

    await userEvent.selectOptions(format(), 'my-player');

    expect(useSync.getState().profile).toBe('my-player');
  });

  test('with the format list focused the keys are the list’s own, not the targets’', async () => {
    await open();
    await userEvent.keyboard('j');
    format().element().focus();

    await userEvent.keyboard('{ArrowDown}');
    await userEvent.keyboard('j');
    await userEvent.keyboard('w');
    await userEvent.keyboard('{Enter}');

    expect(chosenTarget()).toBe('USB');
    expect(requestsTo(fetchMock, 'POST', '/api/sync/plan')).toEqual([]);
    expect(useSync.getState().phase).toBe('choosing');
  });

  test('a click chooses a target and a second click on it goes on', async () => {
    await open();

    await dialog().getByRole('option').nth(1).click();
    expect(chosenTarget()).toBe('USB');
    expect(useSync.getState().phase).toBe('choosing');

    await dialog().getByRole('option').nth(1).click();

    await expect.element(dialog()).toHaveTextContent('Preparing');
    await expect.poll(() => requestsTo(fetchMock, 'POST', '/api/sync/plan')).toEqual([{ library: false, folder: '/media/me/USB', profile: 'generic' }]);
  });

  test('q closes', async () => {
    await open();

    await userEvent.keyboard('q');

    await expect.element(dialog()).not.toBeInTheDocument();
  });

  test('enter on the library folder plans the library, whatever format was left chosen', async () => {
    await open();

    await userEvent.keyboard('{Enter}');

    await expect.poll(() => requestsTo(fetchMock, 'POST', '/api/sync/plan')).toEqual([{ library: true, folder: '', profile: '' }]);
  });
});

describe('typing a folder', () => {
  test('asks for the folder, which is focused, with the formats below it and no w hint', async () => {
    await typing();

    const folder = dialog().getByRole('textbox', { name: 'Folder' });
    await expect.element(folder).toHaveAttribute('placeholder', '~/Music/player');
    await expect.element(folder).toHaveAttribute('dir', 'auto');
    await expect.element(dialog()).toHaveTextContent('Write the music into which folder?');
    await expect.element(format()).toHaveValue('generic');
    await expect.element(dialog()).not.toHaveTextContent('w goes through them');
    await expect.poll(hints).toEqual(['enter plan', 'esc back']);
  });

  test('there is no stray separator after the format when it has nothing to say', async () => {
    await typing();

    expect(dialog().element().querySelector('.format')?.textContent).not.toContain('·');
  });

  test('enter in the format list does not plan the folder', async () => {
    await typing();
    await userEvent.keyboard('~/player');
    format().element().focus();

    await userEvent.keyboard('{Enter}');

    expect(requestsTo(fetchMock, 'POST', '/api/sync/plan')).toEqual([]);
    expect(useSync.getState().phase).toBe('typing');
  });

  test('w is a letter in the folder, not the format key', async () => {
    await typing();

    await userEvent.keyboard('~/w');

    await expect.element(dialog().getByRole('textbox', { name: 'Folder' })).toHaveValue('~/w');
    await expect.element(format()).toHaveValue('generic');
  });

  test('enter with nothing typed complains, and typing takes the complaint away', async () => {
    await typing();

    await userEvent.keyboard('{Enter}');

    await expect.element(dialog()).toHaveTextContent('Type the folder to write into');
    expect(requestsTo(fetchMock, 'POST', '/api/sync/plan')).toEqual([]);
    await userEvent.keyboard('~');
    await expect.element(dialog()).not.toHaveTextContent('Type the folder to write into');
  });

  test('enter plans the folder with the chosen format', async () => {
    await typing();
    await userEvent.selectOptions(format(), 'wolfgang-wmp01');
    await userEvent.click(dialog().getByRole('textbox', { name: 'Folder' }));
    await userEvent.keyboard('~/player{Enter}');

    await expect.poll(() => requestsTo(fetchMock, 'POST', '/api/sync/plan')).toEqual([{ library: false, folder: '~/player', profile: 'wolfgang-wmp01' }]);
  });

  test('esc goes back to the targets instead of closing', async () => {
    await typing();

    await userEvent.keyboard('{Escape}');

    await expect.element(dialog().getByRole('listbox', { name: 'Where to write' })).toBeVisible();
    await expect.element(dialog()).toBeVisible();
  });

  test('a folder the server refuses stays for correction, with the reason', async () => {
    await typing(({ method, url }) => (method === 'POST' && url.pathname === '/api/sync/plan' ? jsonResponse({ error: 'that folder holds the library' }, 422) : undefined));

    await userEvent.keyboard('~{Enter}');

    await expect.element(dialog()).toHaveTextContent('that folder holds the library');
    await expect.element(dialog().getByRole('textbox', { name: 'Folder' })).toHaveValue('~');
    await expect.element(dialog().getByRole('textbox', { name: 'Folder' })).toHaveFocus();
  });
});

describe('planning', () => {
  test('says what it prepares, with esc to cancel, and no bar until it knows how much', async () => {
    await planning();

    await expect.element(dialog()).toHaveTextContent('Preparing USB…');
    await expect.element(dialog()).toHaveTextContent('Checking every file against its record; esc cancels.');
    await expect.element(dialog().getByRole('progressbar')).not.toBeInTheDocument();
    await expect.poll(hints).toEqual(['esc cancel']);
  });

  test('shows a bar as the server reports', async () => {
    await planning();

    emit(job('running', { phase: 'plan', done: 2, total: 8 }));

    const bar = dialog().getByRole('progressbar', { name: 'Checked so far' });
    await expect.element(bar).toHaveAttribute('aria-valuenow', '2');
    await expect.element(bar).toHaveAttribute('aria-valuemax', '8');
  });

  test('esc closes the dialog and stops the plan', async () => {
    await planning();

    await userEvent.keyboard('{Escape}');

    await expect.element(dialog()).not.toBeInTheDocument();
    await expect.poll(() => requestsTo(fetchMock, 'DELETE', '/api/jobs/j1')).toEqual([null]);
  });

  test('a plan that fails shows why', async () => {
    await planning();

    emit(job('failed', undefined, 'the drive is read-only'));

    await expect.element(dialog().getByRole('alert')).toHaveTextContent('the drive is read-only');
    await expect.poll(hints).toEqual(['enter close']);
    await userEvent.keyboard('{Enter}');
    await expect.element(dialog()).not.toBeInTheDocument();
  });

  test('the place is isolated for right-to-left text', async () => {
    await open();
    await userEvent.keyboard('{End}{Enter}');
    await userEvent.keyboard('~/موسيقى{Enter}');

    await expect.element(dialog()).toHaveTextContent('Preparing ~/موسيقى…');
    expect(dialog().element().querySelector('.busy bdi')?.getAttribute('dir')).toBe('auto');
  });
});

describe('the review', () => {
  test('shows the summary and the place, and each file with what would happen to it', async () => {
    await reviewing({ ops: [...ops(1, 'create'), { kind: 'update', path: 'b.mp3' }, { kind: 'remove', path: 'c.mp3' }, { kind: 'stray', path: 'd.mp3' }], counts: counts({ create: 1, update: 1, remove: 1, stray: 1 }), text: '1 new, 1 changed' });

    await expect.element(dialog()).toHaveTextContent('1 new, 1 changed → ~/player');
    const list = dialog().getByRole('list', { name: 'What would change' });
    const items = list.getByRole('listitem').elements();
    expect(items.map((item) => item.textContent.trim())).toEqual(['+Artist/Track 1.mp3', '~b.mp3', '−c.mp3', '!d.mp3']);
    expect(items.map((item) => item.querySelector('[role="img"]')?.getAttribute('aria-label'))).toEqual(['create', 'update', 'remove', 'stray']);
    expect(items.map((item) => item.getAttribute('class'))).toEqual(['op create', 'op update', 'op remove', 'op stray']);
    expect(items.map((item) => item.getAttribute('title'))).toEqual(['Artist/Track 1.mp3', 'b.mp3', 'c.mp3', 'd.mp3']);
    await expect.poll(hints).toEqual(['↑↓ scroll', 'enter apply', 'p strays', 'esc close']);
  });

  test('names, paths and messages are isolated for right-to-left text', async () => {
    await reviewing({ where: '~/موسيقى', ops: [{ kind: 'create', path: 'فيروز/وطني.mp3' }], warnings: [{ track: 'وطني', message: 'رسالة' }] });

    const dirs = [...dialog().element().querySelectorAll('bdi')].map((b) => b.getAttribute('dir'));
    expect(dirs).toEqual(['auto', 'auto', 'auto', 'auto']);
  });

  test('with warnings it says how many, lists the first four and counts the rest', async () => {
    await reviewing({ warnings: warnings(6) });

    await expect.poll(() => dialog().element().querySelector('.warns')?.textContent).toBe('6 warnings for the player');
    await expect.element(dialog()).toHaveTextContent('Track 4');
    await expect.element(dialog()).toHaveTextContent('message 4');
    await expect.element(dialog()).not.toHaveTextContent('Track 5');
    await expect.element(dialog()).toHaveTextContent('… 2 more');
  });

  test('one warning is one warning and nothing more', async () => {
    await reviewing({ warnings: warnings(1) });

    await expect.poll(() => dialog().element().querySelector('.warns')?.textContent).toBe('1 warning for the player');
    await expect.element(dialog()).not.toHaveTextContent('more');
  });

  test('five warnings list four and count one more', async () => {
    await reviewing({ warnings: warnings(5) });

    await expect.element(dialog()).toHaveTextContent('… 1 more');
    await expect.element(dialog()).not.toHaveTextContent('Track 5');
  });

  test('strays and files to delete together show the note about strays, not the one about deleting', async () => {
    await reviewing({ ops: [...ops(1), { kind: 'stray', path: 'd.mp3' }, { kind: 'remove', path: 'e.mp3' }], counts: counts({ stray: 1, remove: 1 }) });

    await expect.element(dialog()).toHaveTextContent('Stray files are left alone');
    await expect.element(dialog()).not.toHaveTextContent('will be deleted');
  });

  test('exactly four warnings are all listed with no more line', async () => {
    await reviewing({ warnings: warnings(4) });

    await expect.element(dialog()).toHaveTextContent('Track 4');
    await expect.element(dialog()).not.toHaveTextContent('… 0 more');
    await expect.element(dialog()).not.toHaveTextContent('more');
  });

  test('without warnings there is no warning line', async () => {
    await reviewing();

    expect(dialog().element().querySelector('.warns')).toBeNull();
  });

  test('strays are left alone unless asked, and the button and p ask', async () => {
    const pruned = plan({ prune: true, ops: [...ops(1), { kind: 'remove', path: 'd.mp3' }], counts: counts({ remove: 1 }) });
    await reviewing({ ops: [...ops(1), { kind: 'stray', path: 'd.mp3' }], counts: counts({ stray: 1 }) }, ({ method, url }) => (method === 'POST' && url.pathname === '/api/sync/p1/prune' ? jsonResponse(pruned) : undefined));
    await expect.element(dialog()).toHaveTextContent('Stray files are left alone. p also deletes the strays.');

    await dialog().getByRole('button', { name: 'Delete the strays' }).click();

    await expect.poll(() => requestsTo(fetchMock, 'POST', '/api/sync/p1/prune')).toEqual([{ prune: true }]);
    await expect.element(dialog()).toHaveTextContent('The files marked − will be deleted. p keeps them.');
    await expect.element(dialog()).not.toHaveTextContent('Stray files are left alone');
    await expect.element(dialog().getByRole('button', { name: 'Keep the strays' })).toBeVisible();
  });

  test('p does the same, and does nothing when there is nothing to write', async () => {
    await reviewing({ ops: [...ops(1), { kind: 'stray', path: 'd.mp3' }], counts: counts({ stray: 1 }) }, ({ method, url }) => (method === 'POST' && url.pathname === '/api/sync/p1/prune' ? jsonResponse(plan({ prune: true, counts: counts({ remove: 1 }) })) : undefined));

    await userEvent.keyboard('p');

    await expect.poll(() => requestsTo(fetchMock, 'POST', '/api/sync/p1/prune')).toEqual([{ prune: true }]);
  });

  test('p is for strays: with nothing to do it asks nothing', async () => {
    await reviewing({ ops: [], counts: counts({ create: 0 }) });

    await userEvent.keyboard('p');

    expect(requestsTo(fetchMock, 'POST', '/api/sync/p1/prune')).toEqual([]);
  });

  test('without strays there is no button for them', async () => {
    await reviewing();

    await expect.element(dialog().getByRole('button', { name: /strays/ })).not.toBeInTheDocument();
    await expect.element(dialog().getByRole('button', { name: 'Apply' })).toBeVisible();
    await expect.element(dialog().getByRole('button', { name: 'Cancel' })).toBeVisible();
  });

  test('a refusal to change the strays is shown', async () => {
    await reviewing({ ops: [...ops(1), { kind: 'stray', path: 'd.mp3' }], counts: counts({ stray: 1 }) }, ({ method, url }) => (method === 'POST' && url.pathname === '/api/sync/p1/prune' ? jsonResponse({ error: 'that plan is gone: prepare the sync again' }, 410) : undefined));

    await userEvent.keyboard('p');

    await expect.element(dialog()).toHaveTextContent('that plan is gone: prepare the sync again');
  });

  test('the list scrolls with the arrows, j and k, the page keys and the ends', async () => {
    await reviewing({ ops: ops(60), counts: counts({ create: 60 }) });
    const list = () => dialog().getByRole('list', { name: 'What would change' }).element();
    const row = () => list().firstElementChild?.getBoundingClientRect().height ?? 0;
    expect(list().scrollTop).toBe(0);

    await userEvent.keyboard('j');
    expect(Math.abs(list().scrollTop - row())).toBeLessThanOrEqual(1);
    await userEvent.keyboard('{ArrowDown}');
    expect(Math.abs(list().scrollTop - 2 * row())).toBeLessThanOrEqual(1);
    await userEvent.keyboard('k');
    expect(Math.abs(list().scrollTop - row())).toBeLessThanOrEqual(1);
    await userEvent.keyboard('{ArrowUp}');
    expect(list().scrollTop).toBe(0);
    await userEvent.keyboard('{PageDown}');
    expect(Math.abs(list().scrollTop - 10 * row())).toBeLessThanOrEqual(1);
    await userEvent.keyboard('{PageUp}');
    expect(list().scrollTop).toBe(0);
    await userEvent.keyboard('{End}');
    expect(list().scrollTop).toBe(list().scrollHeight - list().clientHeight);
    await userEvent.keyboard('{Home}');
    expect(list().scrollTop).toBe(0);
  });

  test('an up to date place has nothing to write, and enter closes', async () => {
    await reviewing({ ops: [], counts: counts({ create: 0 }) });

    await expect.element(dialog()).toHaveTextContent('Nothing to do · ~/player is up to date.');
    await expect.poll(hints).toEqual(['enter close']);
    expect(dialog().element().querySelector('.ops')).toBeNull();
    await userEvent.keyboard('{Enter}');
    await expect.element(dialog()).not.toBeInTheDocument();
  });

  test('enter and the apply button write a plan that deletes nothing at once', async () => {
    await reviewing();

    await userEvent.keyboard('{Enter}');

    await expect.poll(() => requestsTo(fetchMock, 'POST', '/api/sync/p1/apply')).toEqual([{ confirm: '' }]);
  });

  test('cancel closes and drops the plan', async () => {
    await reviewing();

    await dialog().getByRole('button', { name: 'Cancel' }).click();

    await expect.element(dialog()).not.toBeInTheDocument();
    await expect.poll(() => requestsTo(fetchMock, 'DELETE', '/api/sync/p1')).toEqual([null]);
  });

  test('q closes', async () => {
    await reviewing();

    await userEvent.keyboard('q');

    await expect.element(dialog()).not.toBeInTheDocument();
  });
});

describe('confirming a deletion', () => {
  const deleting = { ops: [...ops(1), { kind: 'remove', path: 'old.mp3' }, { kind: 'remove', path: 'older.mp3' }], counts: counts({ remove: 2 }) };

  async function confirming(...extra: StubRoute[]): Promise<void> {
    await reviewing(deleting, ...extra);
    await expect.element(dialog()).toHaveTextContent('The files marked − will be deleted. p keeps them.');
    await userEvent.keyboard('{Enter}');
    await expect.element(dialog().getByRole('textbox', { name: 'Type delete to confirm' })).toHaveFocus();
  }

  test('asks for the word, naming how many files go, with its own buttons and hints', async () => {
    await confirming();

    await expect.element(dialog()).toHaveTextContent('Type delete to remove 2 files:');
    await expect.element(dialog().getByRole('button', { name: 'Delete and write' })).toBeVisible();
    await expect.element(dialog().getByRole('button', { name: 'Back' })).toBeVisible();
    await expect.element(dialog().getByRole('button', { name: 'Apply' })).not.toBeInTheDocument();
    await expect.poll(hints).toEqual(['enter confirm', 'esc back']);
    expect(requestsTo(fetchMock, 'POST', '/api/sync/p1/apply')).toEqual([]);
  });

  test('another word is wiped and nothing is written', async () => {
    await confirming();
    await userEvent.keyboard('delet{Enter}');

    await expect.element(dialog().getByRole('textbox', { name: 'Type delete to confirm' })).toHaveValue('');
    expect(requestsTo(fetchMock, 'POST', '/api/sync/p1/apply')).toEqual([]);
  });

  test('the word writes, and sends the word', async () => {
    await confirming();

    await userEvent.keyboard('delete{Enter}');

    await expect.poll(() => requestsTo(fetchMock, 'POST', '/api/sync/p1/apply')).toEqual([{ confirm: 'delete' }]);
  });

  test('the button needs the word too', async () => {
    await confirming();

    await dialog().getByRole('button', { name: 'Delete and write' }).click();
    expect(requestsTo(fetchMock, 'POST', '/api/sync/p1/apply')).toEqual([]);
    await userEvent.fill(dialog().getByRole('textbox', { name: 'Type delete to confirm' }), 'delete');
    await dialog().getByRole('button', { name: 'Delete and write' }).click();

    await expect.poll(() => requestsTo(fetchMock, 'POST', '/api/sync/p1/apply')).toEqual([{ confirm: 'delete' }]);
  });

  test('esc and the back button return to the review without closing', async () => {
    await confirming();

    await userEvent.keyboard('{Escape}');

    await expect.element(dialog().getByRole('button', { name: 'Apply' })).toBeVisible();
    await userEvent.keyboard('{Enter}');
    await expect.element(dialog().getByRole('textbox', { name: 'Type delete to confirm' })).toBeVisible();
    await dialog().getByRole('button', { name: 'Back' }).click();
    await expect.element(dialog().getByRole('button', { name: 'Apply' })).toBeVisible();
  });

  test('a refusal of the word is shown beside it', async () => {
    await confirming(({ method, url }) => (method === 'POST' && url.pathname === '/api/sync/p1/apply' ? jsonResponse({ error: 'type delete to remove 2 files' }, 422) : undefined));

    await userEvent.keyboard('delete{Enter}');

    await expect.element(dialog().getByText('type delete to remove 2 files', { exact: true })).toBeVisible();
    await expect.element(dialog().getByRole('textbox', { name: 'Type delete to confirm' })).toBeVisible();
  });

  test('one file is a file', async () => {
    await reviewing({ ops: [...ops(1), { kind: 'remove', path: 'old.mp3' }], counts: counts({ remove: 1 }) });

    await userEvent.keyboard('{Enter}');

    await expect.element(dialog()).toHaveTextContent('Type delete to remove 1 file:');
  });
});

describe('writing', () => {
  async function applying(): Promise<void> {
    await reviewing({ ops: ops(4), counts: counts({ create: 4 }) });
    await userEvent.keyboard('{Enter}');
    await expect.element(dialog()).toHaveTextContent('Writing ~/player…');
  }

  test('shows a bar that starts at the number of files and follows the server', async () => {
    await applying();

    const bar = dialog().getByRole('progressbar', { name: 'Written so far' });
    await expect.element(bar).toHaveAttribute('aria-valuemax', '4');
    await expect.element(bar).toHaveAttribute('aria-valuenow', '0');
    emit(job('running', { phase: 'apply', done: 3, total: 4 }));
    await expect.element(bar).toHaveAttribute('aria-valuenow', '3');
    await expect.poll(hints).toEqual([]);
  });

  test('there is nothing to press while it writes', async () => {
    await applying();

    await userEvent.keyboard('{Enter}');
    await userEvent.keyboard('q');
    await userEvent.keyboard('p');

    await expect.element(dialog()).toHaveTextContent('Writing ~/player…');
    expect(requestsTo(fetchMock, 'POST', '/api/sync/p1/apply')).toHaveLength(1);
  });

  test('ends with Done, the place and what was written, and enter closes', async () => {
    await applying();

    emit(job('done', { phase: 'apply', summary: '4 new' }));

    await expect.element(dialog()).toHaveTextContent('Done ~/player');
    await expect.element(dialog()).toHaveTextContent('4 new');
    await expect.poll(hints).toEqual(['enter close']);
    await userEvent.keyboard('{Enter}');
    await expect.element(dialog()).not.toBeInTheDocument();
  });

  test('a write that fails shows why', async () => {
    await applying();

    emit(job('failed', undefined, 'no space left'));

    await expect.element(dialog().getByRole('alert')).toHaveTextContent('no space left');
    await userEvent.keyboard('x');
    await expect.element(dialog()).toBeVisible();
    await userEvent.keyboard('{Enter}');
    await expect.element(dialog()).not.toBeInTheDocument();
  });

  test('closing the dialog while it writes lets it finish and says so in the status line', async () => {
    await applying();

    await userEvent.keyboard('{Escape}');
    await expect.element(dialog()).not.toBeInTheDocument();
    emit(job('done', { phase: 'apply', summary: '4 new' }));

    await expect.element(page.getByRole('status')).toHaveTextContent('Synced to USB: 4 new');
    expect(requestsTo(fetchMock, 'DELETE', '/api/jobs/j1')).toEqual([]);
  });
});
