import { create } from 'zustand';
import { get, send } from '../api/client';
import type { JobView, PlanView, SyncData, SyncTargetsView, TargetView } from '../api/types.gen';
import { loadLibrary } from './library';
import { fail, setStatus } from './status';
import { follow, stopJob } from './jobs';
import { useApp } from './store';

export type SyncPhase = 'choosing' | 'typing' | 'planning' | 'reviewing' | 'confirming' | 'applying' | 'finished' | 'failed';

export interface SyncState {
  phase: SyncPhase;
  targets: TargetView[];
  cursor: number;
  profile: string;
  folder: string;
  error: string;
  where: string;
  job: JobView | null;
  plan: PlanView | null;
  summary: string;
  confirm: string;
}

export const genericProfile = 'generic';

const anotherFolder: TargetView = { label: 'Another folder…', path: '', kind: 'folder' };
const stopped = 'The sync was stopped.';

function fresh(): SyncState {
  return {
    phase: 'choosing',
    targets: [],
    cursor: 0,
    profile: genericProfile,
    folder: '',
    error: '',
    where: '',
    job: null,
    plan: null,
    summary: '',
    confirm: '',
  };
}

export const useSync = create<SyncState>()(fresh);

let session = 0;

function messageOf(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

function target(): TargetView | undefined {
  const { targets, cursor } = useSync.getState();
  return targets[cursor];
}

export function isLibraryTarget(): boolean {
  return target()?.kind === 'library';
}

function isSyncData(data: unknown): data is SyncData {
  return typeof data === 'object' && data !== null && 'phase' in data && typeof data.phase === 'string';
}

export function progressOf(job: JobView | null, phase: 'plan' | 'apply', total: number): { done: number; total: number } {
  if (job !== null && isSyncData(job.data) && job.data.phase === phase) {
    return { done: job.data.done ?? 0, total: job.data.total ?? 0 };
  }
  return { done: 0, total };
}

export async function openSync(): Promise<void> {
  if (useApp.getState().lib === null) {
    return;
  }
  let found: SyncTargetsView;
  try {
    found = await get<SyncTargetsView>('/api/sync/targets');
  } catch (err) {
    fail(err);
    return;
  }
  if (useApp.getState().overlay !== null) {
    return;
  }
  session += 1;
  useSync.setState({ ...fresh(), targets: [...found.targets, anotherFolder] });
  useApp.setState({ overlay: { kind: 'sync' } });
}

export function chooseTarget(i: number): void {
  useSync.setState((s) => ({ cursor: Math.min(Math.max(i, 0), s.targets.length - 1) }));
}

export function moveTarget(by: number): void {
  chooseTarget(useSync.getState().cursor + by);
}

export function chooseProfile(name: string): void {
  useSync.setState({ profile: name });
}

export function cycleProfile(): void {
  if (isLibraryTarget()) {
    return;
  }
  const names = useApp.getState().profiles.map((p) => p.name);
  const at = names.indexOf(useSync.getState().profile);
  const next = names[(at + 1) % names.length];
  if (next !== undefined) {
    useSync.setState({ profile: next });
  }
}

export function typeFolder(text: string): void {
  useSync.setState({ folder: text, error: '' });
}

export function backToChoosing(): void {
  useSync.setState({ phase: 'choosing', error: '' });
}

export function pickTarget(): void {
  if (target()?.kind === 'folder') {
    useSync.setState({ phase: 'typing', error: '' });
    return;
  }
  void startPlan();
}

export function submitFolder(): void {
  if (useSync.getState().folder.trim() === '') {
    useSync.setState({ error: 'Type the folder to write into' });
    return;
  }
  void startPlan();
}

async function startPlan(): Promise<void> {
  const picked = target();
  if (picked === undefined) {
    return;
  }
  const { folder, profile } = useSync.getState();
  const typed = picked.kind === 'folder';
  const library = picked.kind === 'library';
  const mine = session;
  useSync.setState({ phase: 'planning', error: '', where: typed ? folder.trim() : picked.label, job: null });
  let started: JobView;
  try {
    started = await send<JobView>('POST', '/api/sync/plan', {
      library,
      folder: typed ? folder : picked.kind === 'drive' ? picked.path : '',
      profile: library ? '' : profile,
    });
  } catch (err) {
    if (mine === session) {
      useSync.setState({ phase: typed ? 'typing' : 'failed', error: messageOf(err) });
    }
    return;
  }
  if (mine !== session) {
    void stopJob(started.id);
    return;
  }
  useSync.setState({ job: started });
  const ended = await follow(started, (v) => {
    if (mine === session) {
      useSync.setState({ job: v });
    }
  });
  if (mine !== session) {
    return;
  }
  const plan = isSyncData(ended.data) ? ended.data.plan : undefined;
  if (plan === undefined) {
    useSync.setState({ phase: 'failed', error: ended.error ?? stopped });
    return;
  }
  useSync.setState({ phase: 'reviewing', plan });
  if (library && plan.ops.length === 0) {
    void loadLibrary();
  }
}

export async function togglePrune(): Promise<void> {
  const { plan } = useSync.getState();
  if (plan === null) {
    return;
  }
  const mine = session;
  try {
    const next = await send<PlanView>('POST', `/api/sync/${encodeURIComponent(plan.plan)}/prune`, { prune: !plan.prune });
    if (mine === session) {
      useSync.setState({ plan: next, error: '' });
    }
  } catch (err) {
    if (mine === session) {
      useSync.setState({ error: messageOf(err) });
    }
  }
}

export function applyPlan(): void {
  const { plan } = useSync.getState();
  if (plan === null) {
    return;
  }
  if (plan.ops.length === 0) {
    useApp.setState({ overlay: null });
    return;
  }
  if (plan.counts.remove > 0) {
    useSync.setState({ phase: 'confirming', confirm: '', error: '' });
    return;
  }
  void writePlan();
}

export function typeConfirm(text: string): void {
  useSync.setState({ confirm: text });
}

export function confirmDelete(): void {
  if (useSync.getState().confirm.trim() === 'delete') {
    void writePlan();
    return;
  }
  useSync.setState({ confirm: '' });
}

export function backToReview(): void {
  useSync.setState({ phase: 'reviewing', error: '' });
}

async function writePlan(): Promise<void> {
  const { plan, confirm, where } = useSync.getState();
  if (plan === null) {
    return;
  }
  const mine = session;
  useSync.setState({ phase: 'applying', error: '' });
  let started: JobView;
  try {
    started = await send<JobView>('POST', `/api/sync/${encodeURIComponent(plan.plan)}/apply`, { confirm: confirm.trim() });
  } catch (err) {
    if (mine === session) {
      useSync.setState({ phase: plan.counts.remove > 0 ? 'confirming' : 'failed', error: messageOf(err) });
    }
    return;
  }
  useSync.setState({ job: started });
  const ended = await follow(started, (v) => {
    if (mine === session) {
      useSync.setState({ job: v });
    }
  });
  const summary = isSyncData(ended.data) ? (ended.data.summary ?? '') : '';
  const error = ended.error ?? stopped;
  if (mine === session) {
    useSync.setState(ended.state === 'done' ? { phase: 'finished', summary } : { phase: 'failed', error });
  }
  await loadLibrary();
  if (mine !== session) {
    if (ended.state === 'done') {
      setStatus(`Synced to ${where}: ${summary}`, 'good');
    } else {
      setStatus(error, 'bad');
    }
  }
}

useApp.subscribe((state, previous) => {
  if (previous.overlay?.kind !== 'sync' || state.overlay?.kind === 'sync') {
    return;
  }
  session += 1;
  const { phase, job, plan } = useSync.getState();
  if (phase === 'planning' && job !== null) {
    void stopJob(job.id);
  }
  if ((phase === 'reviewing' || phase === 'confirming') && plan !== null) {
    send('DELETE', `/api/sync/${encodeURIComponent(plan.plan)}`).catch(() => undefined);
  }
});
