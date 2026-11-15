import { readdirSync, readFileSync } from 'node:fs';
import { basename, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const sourceRoot = fileURLToPath(new URL('..', import.meta.url));
const opener = '/' + '*';

function stylesheets(): string[] {
  return readdirSync(sourceRoot, { recursive: true, encoding: 'utf8' })
    .filter((path) => path.endsWith('.css'))
    .map((path) => join(sourceRoot, path));
}

describe('the stylesheets', () => {
  it('include the tokens and the application styles', () => {
    expect(stylesheets().map((path) => basename(path))).toEqual(expect.arrayContaining(['app.css', 'tokens.css']));
  });

  it('contain no comments', () => {
    const commented = stylesheets().filter((path) => readFileSync(path, 'utf8').includes(opener));

    expect(commented).toEqual([]);
  });
});
