import { useApp, type AppState } from './store';
import { messageOf } from './text';

export function setStatus(text: string, kind: AppState['status']['kind'] = ''): void {
  useApp.setState({ status: { text, kind } });
}

export function fail(err: unknown): void {
  setStatus(messageOf(err), 'bad');
}
