import { create } from 'zustand';
import { ApiError, send } from '../api/client';
import type { Profile, ProfileCheck } from '../api/types.gen';
import { cleanChecks, firstWrong, left, tried, verdict, type FormChecks } from './form';
import { loadLibrary } from './library';
import { setStatus } from './status';
import { ask, useApp } from './store';

export type FormatsMode = 'list' | 'form';

export interface FormatForm {
  name: string;
  latin_names: boolean;
  component_max: string;
  stem_max: string;
  tag_width: string;
  latin_genre: boolean;
  cover_format: string;
  cover_max_edge: string;
  cover_quality: string;
  single_covers: boolean;
  replaygain: boolean;
  playlist_ext: string;
  playlist_style: string;
}

export type FormatKey = keyof FormatForm;

export const formatOrder: readonly FormatKey[] = [
  'name',
  'latin_names',
  'component_max',
  'stem_max',
  'tag_width',
  'latin_genre',
  'cover_format',
  'cover_max_edge',
  'cover_quality',
  'single_covers',
  'replaygain',
  'playlist_ext',
  'playlist_style',
];

export const maxFormats = 8;

export interface FormatsState {
  mode: FormatsMode;
  cursor: number;
  editing: string | null;
  form: FormatForm;
  checks: FormChecks;
  error: string;
  saving: boolean;
}

const checkPause = 150;
const enc = encodeURIComponent;

export function blankForm(): FormatForm {
  return {
    name: '',
    latin_names: false,
    component_max: '200',
    stem_max: '0',
    tag_width: '0',
    latin_genre: false,
    cover_format: 'keep',
    cover_max_edge: '0',
    cover_quality: '0',
    single_covers: false,
    replaygain: true,
    playlist_ext: '.m3u8',
    playlist_style: 'relative',
  };
}

export function formOf(p: Profile): FormatForm {
  return {
    name: p.name,
    latin_names: p.latin_names,
    component_max: String(p.component_max),
    stem_max: String(p.stem_max),
    tag_width: String(p.tag_width),
    latin_genre: p.latin_genre,
    cover_format: p.cover_format,
    cover_max_edge: String(p.cover_max_edge),
    cover_quality: String(p.cover_quality),
    single_covers: p.single_covers,
    replaygain: p.replaygain,
    playlist_ext: p.playlist_ext,
    playlist_style: p.playlist_style,
  };
}

export function tagWidthOn(form: FormatForm): boolean {
  return form.latin_names;
}

export function coverSizeOn(form: FormatForm): boolean {
  return form.cover_format === 'baseline-jpeg';
}

const notWhole = 'Must be a whole number';

export function parseForm(form: FormatForm): { profile: Profile; local: Record<string, string> } {
  const local: Record<string, string> = {};
  const count = (key: FormatKey, on = true): number => {
    const text = String(form[key]).trim();
    if (!on) {
      return 0;
    }
    if (!/^\d+$/.test(text)) {
      local[key] = notWhole;
      return 0;
    }
    return Number(text);
  };
  const profile: Profile = {
    id: '',
    name: form.name,
    latin_names: form.latin_names,
    component_max: count('component_max'),
    stem_max: count('stem_max'),
    tag_width: count('tag_width', tagWidthOn(form)),
    latin_genre: form.latin_genre,
    cover_format: form.cover_format,
    cover_max_edge: count('cover_max_edge', coverSizeOn(form)),
    cover_quality: count('cover_quality', coverSizeOn(form)),
    single_covers: form.single_covers,
    replaygain: form.replaygain,
    playlist_ext: form.playlist_ext,
    playlist_style: form.playlist_style,
  };
  return { profile, local };
}

function fresh(): FormatsState {
  return { mode: 'list', cursor: 0, editing: null, form: blankForm(), checks: cleanChecks(), error: '', saving: false };
}

export const useFormats = create<FormatsState>()(fresh);

let timer: ReturnType<typeof setTimeout> | undefined;
let seq = 0;

function stop(): void {
  clearTimeout(timer);
  seq += 1;
}

function stored(): Profile[] {
  return useApp.getState().lib?.profiles ?? [];
}

function open(cursor: number): void {
  stop();
  useFormats.setState({ ...fresh(), cursor });
  useApp.setState({ overlay: { kind: 'formats' } });
}

export function openFormats(): void {
  open(0);
}

export function moveFormats(to: number): void {
  useFormats.setState({ cursor: Math.min(Math.max(to, 0), stored().length) });
}

function messageOf(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

function scheduleCheck(): void {
  clearTimeout(timer);
  timer = setTimeout(() => {
    void runCheck();
  }, checkPause);
}

async function runCheck(): Promise<void> {
  const { form, editing } = useFormats.getState();
  const { profile, local } = parseForm(form);
  seq += 1;
  const mine = seq;
  try {
    const res = await send<ProfileCheck>('POST', '/api/profiles/check', { ...profile, id: editing ?? '' });
    if (mine === seq && useApp.getState().overlay?.kind === 'formats') {
      useFormats.setState((s) => ({ checks: verdict(s.checks, { ...(res.fields ?? {}), ...local }) }));
    }
  } catch {
    return;
  }
}

function enter(editing: string | null, form: FormatForm): void {
  useFormats.setState({ mode: 'form', editing, form, checks: cleanChecks(), error: '' });
  scheduleCheck();
}

export function newFormat(): void {
  if (stored().length >= maxFormats) {
    setStatus(`There can be at most ${String(maxFormats)} formats besides Generic: delete one first`, 'bad');
    return;
  }
  enter(null, blankForm());
}

export function editFormat(): void {
  const chosen = stored()[useFormats.getState().cursor - 1];
  if (chosen === undefined) {
    setStatus('Generic is the standard format and cannot be changed');
    return;
  }
  enter(chosen.id, formOf(chosen));
}

export function backToList(): void {
  stop();
  useFormats.setState({ mode: 'list', editing: null, checks: cleanChecks(), error: '' });
}

export function typeFormat<K extends FormatKey>(key: K, value: FormatForm[K]): void {
  useFormats.setState((s) => ({ form: { ...s.form, [key]: value }, error: '' }));
  scheduleCheck();
}

export function leaveFormat(key: FormatKey): void {
  useFormats.setState((s) => ({ checks: left(s.checks, key) }));
}

function firstKey(): FormatKey | undefined {
  const first = firstWrong(useFormats.getState().checks, formatOrder);
  return formatOrder.find((key) => key === first);
}

export async function saveFormat(): Promise<FormatKey | undefined> {
  const { editing, form, saving, cursor } = useFormats.getState();
  if (saving) {
    return undefined;
  }
  const { profile, local } = parseForm(form);
  if (Object.keys(local).length > 0) {
    useFormats.setState((s) => ({ checks: tried(verdict(s.checks, { ...s.checks.wrong, ...local })) }));
    return firstKey();
  }
  useFormats.setState({ saving: true, error: '' });
  try {
    const made =
      editing === null ? await send<Profile>('POST', '/api/profiles', profile) : await send<Profile>('PUT', `/api/profiles/${enc(editing)}`, profile);
    await loadLibrary();
    setStatus(`Saved the format “${made.name}”`, 'good');
    const at = stored().findIndex((p) => p.id === made.id);
    open(at < 0 ? cursor : at + 1);
  } catch (err) {
    if (err instanceof ApiError && Object.keys(err.fields).length > 0) {
      const fields = err.fields;
      useFormats.setState((s) => ({ saving: false, checks: tried(verdict(s.checks, fields)) }));
      return firstKey();
    }
    useFormats.setState({ saving: false, error: messageOf(err) });
  }
  return undefined;
}

export function removeFormat(): void {
  const at = useFormats.getState().cursor;
  const chosen = stored()[at - 1];
  if (chosen === undefined) {
    setStatus('Generic is the standard format and cannot be deleted');
    return;
  }
  ask(
    `Delete the format “${chosen.name}”?`,
    ['Nothing already written to a player is touched.', 'There is no undo.'],
    'delete',
    () => {
      void deleteFormat(chosen, at);
    },
  );
}

async function deleteFormat(chosen: Profile, at: number): Promise<void> {
  try {
    await send('DELETE', `/api/profiles/${enc(chosen.id)}`);
    await loadLibrary();
    setStatus(`Deleted the format “${chosen.name}”`, 'good');
  } catch (err) {
    setStatus(messageOf(err), 'bad');
  }
  open(Math.max(at - 1, 0));
}

useApp.subscribe((state, previous) => {
  if (previous.overlay?.kind === 'formats' && state.overlay?.kind !== 'formats') {
    stop();
  }
});
