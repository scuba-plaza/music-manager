import { create } from 'zustand';

export interface Hint {
  keys: string;
  label: string;
}

export const useDialogHints = create<{ hints: readonly Hint[] }>()(() => ({ hints: [] }));
