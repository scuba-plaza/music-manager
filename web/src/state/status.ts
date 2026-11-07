import { useApp, type AppState } from './store';

export function setStatus(text: string, kind: AppState['status']['kind'] = ''): void {
  useApp.setState({ status: { text, kind } });
}

export function fail(err: unknown): void {
  setStatus(err instanceof Error ? err.message : String(err), 'bad');
}
