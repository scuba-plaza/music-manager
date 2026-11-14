import { send } from '../api/client';
import type { ChangeResponse } from '../api/types.gen';
import type { ActionName } from '../keys/keymap';
import { setStatus } from './status';
import { openAdd } from './add';
import { openDoctor } from './doctor';
import { attempt } from './edit';
import { identify } from './identify';
import { identifyAll } from './identifyAll';
import { useApp } from './store';
import { idsOf, markedIds, rowByKey } from './tree';

export function adopt(): void {
  const { lib, cursor, marks } = useApp.getState();
  const row = rowByKey(lib, cursor);
  let ids: string[] = [];
  if (lib !== null) {
    if (marks.size > 0) {
      ids = markedIds(lib.view.rows, marks);
    } else if (row !== undefined && (row.kind === 'track' || row.key === 'needs')) {
      ids = idsOf(lib.view.rows, row);
    }
  }
  if (ids.length === 0) {
    setStatus('Select a track that needs work, or the Needs work group', 'bad');
    return;
  }
  void attempt(() => send<ChangeResponse>('POST', '/api/adopt', { ids }));
}

export function runLookup(action: ActionName): boolean {
  switch (action) {
    case 'identify':
      identify();
      return true;
    case 'identifyAll':
      identifyAll();
      return true;
    case 'adopt':
      adopt();
      return true;
    case 'add':
      openAdd();
      return true;
    case 'doctor':
      openDoctor();
      return true;
    default:
      return false;
  }
}
