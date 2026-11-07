import { send } from '../api/client';
import type { ChangeResponse } from '../api/types.gen';
import { setStatus } from './status';
import { attempt } from './edit';
import { plural } from './text';

const mp3Name = /\.mp3$/i;

export function hasMp3(files: readonly File[]): boolean {
  return files.some((f) => mp3Name.test(f.name));
}

export async function uploadFiles(files: readonly File[]): Promise<void> {
  const mp3 = files.filter((f) => mp3Name.test(f.name));
  if (mp3.length === 0) {
    setStatus('Only MP3 files can be added', 'bad');
    return;
  }
  const body = new FormData();
  for (const f of mp3) {
    body.append('files', f);
  }
  setStatus(`Adding ${plural(mp3.length, 'file')}…`);
  await attempt(() => send<ChangeResponse>('POST', '/api/ingest', body));
}
