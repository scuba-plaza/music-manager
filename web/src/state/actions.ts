import { get, query } from '../api/client';
import { subscribe } from '../api/events';
import type { InfoView, JobView, LibraryResponse } from '../api/types.gen';
import type { ActionName } from '../keys/keymap';
import { useApp, type AppState, type Focus } from './store';

export function setStatus(text: string, kind: AppState['status']['kind'] = ''): void {
  useApp.setState({ status: { text, kind } });
}

export function fail(err: unknown): void {
  setStatus(err instanceof Error ? err.message : String(err), 'bad');
}

export async function loadInfo(): Promise<void> {
  try {
    useApp.setState({ info: await get<InfoView>('/api/info') });
  } catch (err) {
    fail(err);
  }
}

let librarySeq = 0;

export async function loadLibrary(): Promise<void> {
  librarySeq += 1;
  const seq = librarySeq;
  const { filter, sort } = useApp.getState();
  try {
    const lib = await get<LibraryResponse>(
      '/api/library' + query({ q: filter.trim(), sort: sort.key, reverse: sort.key !== '' && sort.reverse }),
    );
    if (seq !== librarySeq) {
      return;
    }
    useApp.setState({ lib, sort: { key: lib.view.sort.key, reverse: lib.view.sort.reverse } });
  } catch (err) {
    if (seq === librarySeq) {
      fail(err);
    }
  }
}

export function connect(onJob: (job: JobView) => void): () => void {
  return subscribe({
    library(stamp) {
      const { lib, busy } = useApp.getState();
      if (stamp !== lib?.stamp && !busy) {
        void loadLibrary();
      }
    },
    job: onJob,
    open() {
      if (useApp.getState().lib) {
        void loadLibrary();
      }
    },
  });
}

function focusPane(focus: Focus): void {
  useApp.setState({ focus });
}

export function run(action: ActionName): void {
  const { focus, status } = useApp.getState();
  if (status.text !== '') {
    setStatus('');
  }
  switch (action) {
    case 'open':
      focusPane('inspector');
      break;
    case 'previousTabOrBack':
    case 'escape':
      focusPane('tree');
      break;
    case 'toggleFocus':
      focusPane(focus === 'tree' ? 'inspector' : 'tree');
      break;
    default:
      break;
  }
}
