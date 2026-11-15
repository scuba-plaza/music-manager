export function plural(n: number, word: string): string {
  if (n === 1) {
    return `1 ${word}`;
  }
  return `${String(n)} ${word}${/(ch|sh|s|x)$/.test(word) ? 'es' : 's'}`;
}

export function messageOf(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}
