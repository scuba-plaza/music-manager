import { describe, expect, it } from 'vitest';
import type { Profile } from '../api/types.gen';
import { sampleProfiles } from '../testing/fakes';
import { formatChoices, genericId, previewChoice } from './profiles';

const [wolfgang, myPlayer] = sampleProfiles() as [Profile, Profile];

describe('which format the Device tab previews', () => {
  it('is none when the library has no format of its own', () => {
    expect(previewChoice([], '')).toBe('');
    expect(previewChoice([], genericId)).toBe('');
    expect(previewChoice([], 'gone')).toBe('');
  });

  it('is the first format of the library until another is chosen', () => {
    expect(previewChoice([wolfgang, myPlayer], '')).toBe(wolfgang.id);
  });

  it('is the one chosen, while it is there, Generic included', () => {
    expect(previewChoice([wolfgang, myPlayer], myPlayer.id)).toBe(myPlayer.id);
    expect(previewChoice([wolfgang, myPlayer], genericId)).toBe(genericId);
  });

  it('goes back to the first format when the one chosen is gone', () => {
    expect(previewChoice([wolfgang], myPlayer.id)).toBe(wolfgang.id);
  });
});

describe('the formats a sync or a preview may choose', () => {
  it('are Generic first, then the formats of the library in their order', () => {
    expect(formatChoices([wolfgang, myPlayer])).toEqual([
      { id: 'generic', name: 'Generic' },
      { id: wolfgang.id, name: wolfgang.name },
      { id: myPlayer.id, name: myPlayer.name },
    ]);
  });

  it('are Generic alone for a library with no format of its own', () => {
    expect(formatChoices([])).toEqual([{ id: 'generic', name: 'Generic' }]);
  });
});
