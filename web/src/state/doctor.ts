import { create } from 'zustand';
import { get } from '../api/client';
import type { DoctorView } from '../api/types.gen';
import { loadLibrary } from './actions';
import { useApp } from './store';

export type DoctorPhase = 'checking' | 'done' | 'failed';

export interface DoctorState {
  phase: DoctorPhase;
  result: DoctorView | null;
  error: string;
  cursor: number;
}

function fresh(): DoctorState {
  return { phase: 'checking', result: null, error: '', cursor: 0 };
}

export const useDoctor = create<DoctorState>()(fresh);

let current: AbortController | null = null;

export function openDoctor(): void {
  current?.abort();
  const mine = new AbortController();
  current = mine;
  useDoctor.setState(fresh());
  useApp.setState({ overlay: { kind: 'doctor' } });
  get<DoctorView>('/api/doctor', { signal: mine.signal }).then(
    (result) => {
      if (current === mine) {
        useDoctor.setState({ phase: 'done', result });
      }
    },
    (err: unknown) => {
      if (current === mine) {
        useDoctor.setState({ phase: 'failed', error: err instanceof Error ? err.message : String(err) });
      }
    },
  );
}

export function chooseProblem(i: number): void {
  useDoctor.setState((s) => ({ cursor: Math.min(Math.max(i, 0), Math.max((s.result?.problems.length ?? 0) - 1, 0)) }));
}

export function moveProblem(by: number): void {
  chooseProblem(useDoctor.getState().cursor + by);
}

export function goToProblem(): void {
  const { result, cursor } = useDoctor.getState();
  const problem = result?.problems[cursor];
  if (problem === undefined) {
    return;
  }
  useApp.setState({ overlay: null });
  void loadLibrary({ reveal: problem.trackId });
}

useApp.subscribe((state, previous) => {
  if (previous.overlay?.kind === 'doctor' && state.overlay?.kind !== 'doctor') {
    current?.abort();
    current = null;
  }
});
