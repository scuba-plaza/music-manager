import { query, send } from '../api/client';
import type { ChangeResponse } from '../api/types.gen';
import { setStatus } from './actions';
import { attempt } from './edit';
import { currentField, isTrack } from './inspector';
import { useApp } from './store';
import { isAlbum, markedIds, rowByKey } from './tree';

export interface CoverTarget {
  ids?: string[];
  group?: string;
}

export const coverChooserAttribute = 'data-cover-chooser';

const pictureType = /^image\/(jpeg|png)$/;
const inputTags = new Set(['INPUT', 'TEXTAREA', 'SELECT']);

export function coverTarget(fromPane: boolean): CoverTarget | null {
  const { lib, cursor, marks } = useApp.getState();
  if (!fromPane && lib !== null && marks.size > 0) {
    return { ids: markedIds(lib.view.rows, marks) };
  }
  const row = rowByKey(lib, cursor);
  if (row?.kind === 'track' && row.trackId !== undefined) {
    return { ids: [row.trackId] };
  }
  if (row !== undefined && (row.kind === 'artist' || isAlbum(row))) {
    return { group: row.key };
  }
  return null;
}

function coverQuery(target: CoverTarget, extra: { paste?: boolean; name?: string } = {}): string {
  return query({ ...extra, ids: target.ids?.join(',') ?? '', group: target.group ?? '' });
}

export async function setCover(file: File, target: CoverTarget | null, paste = false): Promise<void> {
  if (target === null) {
    setStatus('Select a track, an album or an artist for the cover', 'bad');
    return;
  }
  if (!pictureType.test(file.type)) {
    setStatus('A cover must be a JPEG or PNG image', 'bad');
    return;
  }
  const extra = paste ? { paste: true, name: file.name } : {};
  await attempt(() => send<ChangeResponse>('PUT', '/api/cover' + coverQuery(target, extra), file));
}

export function chooseCover(): void {
  document.querySelector<HTMLInputElement>(`[${coverChooserAttribute}]`)?.click();
}

export function coverChosen(file: File | undefined): void {
  if (file !== undefined) {
    void setCover(file, coverTarget(true));
  }
}

export function removeCover(): void {
  const s = useApp.getState();
  const d = s.detail;
  const target = coverTarget(true);
  if (currentField(s)?.key !== 'cover' || d === null || target === null) {
    return;
  }
  const had = isTrack(d) ? (d.coverSha ?? '') !== '' : (d.coverTrack ?? '') !== '';
  if (!had) {
    setStatus('There is no cover to remove');
    return;
  }
  void attempt(() => send<ChangeResponse>('DELETE', '/api/cover' + coverQuery(target)));
}

function pictureIn(files: readonly File[]): File | undefined {
  return files.find((f) => pictureType.test(f.type));
}

export async function clipboardPicture(): Promise<File | undefined> {
  try {
    for (const item of await navigator.clipboard.read()) {
      const type = item.types.find((t) => pictureType.test(t));
      if (type !== undefined) {
        return new File([await item.getType(type)], `clipboard.${type.slice('image/'.length)}`, { type });
      }
    }
  } catch {
    return undefined;
  }
  return undefined;
}

export async function pastePicture(file: File): Promise<void> {
  await setCover(file, coverTarget(false), true);
}

export async function dropFiles(files: readonly File[]): Promise<void> {
  const image = pictureIn(files) ?? files.find((f) => f.type.startsWith('image/'));
  if (image === undefined) {
    setStatus('Drop MP3 files to add them, or a JPEG or PNG picture to make it a cover', 'bad');
    return;
  }
  await setCover(image, coverTarget(false));
}

export function installPictures(doc: Document): () => void {
  const open = () => useApp.getState().overlay !== null;
  const carries = (e: DragEvent) => e.dataTransfer?.types.includes('Files') === true;
  const onPaste = (e: ClipboardEvent) => {
    if (open() || inputTags.has(doc.activeElement?.tagName ?? '')) {
      return;
    }
    const files = [...(e.clipboardData?.files ?? [])];
    const file = pictureIn(files) ?? files[0];
    if (file === undefined) {
      return;
    }
    e.preventDefault();
    void pastePicture(file);
  };
  const onDragOver = (e: DragEvent) => {
    if (!carries(e) || open()) {
      return;
    }
    e.preventDefault();
    doc.body.classList.add('dropping');
  };
  const onDragLeave = (e: DragEvent) => {
    if (e.relatedTarget === null) {
      doc.body.classList.remove('dropping');
    }
  };
  const onDrop = (e: DragEvent) => {
    if (!carries(e)) {
      return;
    }
    e.preventDefault();
    doc.body.classList.remove('dropping');
    if (!open()) {
      void dropFiles([...(e.dataTransfer?.files ?? [])]);
    }
  };
  doc.addEventListener('paste', onPaste);
  doc.addEventListener('dragover', onDragOver);
  doc.addEventListener('dragleave', onDragLeave);
  doc.addEventListener('drop', onDrop);
  return () => {
    doc.removeEventListener('paste', onPaste);
    doc.removeEventListener('dragover', onDragOver);
    doc.removeEventListener('dragleave', onDragLeave);
    doc.removeEventListener('drop', onDrop);
  };
}
