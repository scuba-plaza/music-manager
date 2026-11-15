import { useEffect, useRef, type ChangeEvent, type ReactNode } from 'react';
import type { Profile } from '../api/types.gen';
import { FormField } from '../components/FormField';
import { motionOf, moveTo } from '../keys/dialog';
import type { KeyEventLike } from '../keys/keymap';
import { Dialog } from '../overlay/Dialog';
import { ringKey } from '../overlay/focusRing';
import { MenuList } from '../overlay/MenuList';
import {
  backToList,
  coverSizeOn,
  editFormat,
  leaveFormat,
  moveFormats,
  newFormat,
  removeFormat,
  saveFormat,
  tagWidthOn,
  typeFormat,
  useFormats,
  type FormatForm,
  type FormatKey,
} from '../state/formats';
import { shownErrors } from '../state/form';
import { useApp } from '../state/store';

const listHints = [
  { keys: 'enter', label: 'edit' },
  { keys: 'c', label: 'new' },
  { keys: 'd', label: 'delete' },
  { keys: 'esc', label: 'close' },
];

const formHints = [
  { keys: 'enter', label: 'save' },
  { keys: 'tab', label: 'next' },
  { keys: 'esc', label: 'back' },
];

const noFormats: Profile[] = [];

function summary(p: Profile): string {
  return [p.latin_names ? 'Latin letters' : '', `names up to ${String(p.component_max)}`, p.playlist_ext].filter((s) => s !== '').join(' · ');
}

function Group({ name }: { name: string }) {
  return <div className="group">{name}</div>;
}

export function Formats() {
  const stored = useApp((s) => s.lib?.profiles ?? noFormats);
  const { mode, cursor, editing, form, checks, error } = useFormats();
  const errors = shownErrors(checks);
  const grid = useRef<HTMLDivElement>(null);

  function controls(): HTMLElement[] {
    return [...(grid.current?.querySelectorAll<HTMLElement>('input:not(:disabled), select:not(:disabled)') ?? [])];
  }

  useEffect(() => {
    if (mode === 'form') {
      controls()[0]?.focus();
    }
  }, [mode]);

  function save(): void {
    void saveFormat().then((wrong) => {
      if (wrong !== undefined) {
        grid.current?.querySelector<HTMLElement>(`[data-key="${wrong}"]`)?.focus();
      }
    });
  }

  function onKey(e: KeyEventLike & { target?: EventTarget | null }): boolean {
    if (mode === 'list') {
      const motion = motionOf(e);
      if (motion !== undefined) {
        moveFormats(moveTo(stored.length + 1, cursor, motion));
      } else if (e.key === 'Enter') {
        editFormat();
      } else if (e.key === 'c') {
        newFormat();
      } else if (e.key === 'd' || e.key === 'Delete') {
        removeFormat();
      } else {
        return false;
      }
      return true;
    }
    const onSelect = document.activeElement instanceof HTMLSelectElement;
    const onText = document.activeElement instanceof HTMLInputElement && document.activeElement.type === 'text';
    if (e.key === 'Enter') {
      save();
    } else if (e.key === 'Escape' || (e.key === 'q' && !onText)) {
      backToList();
    } else {
      return ringKey(e, controls(), !onSelect);
    }
    return true;
  }

  function text(key: FormatKey, label: string, hint?: string, extra: { disabled?: boolean; numeric?: boolean } = {}): ReactNode {
    return (
      <FormField key={key} name={key} label={label} error={errors[key]} hint={hint}>
        {(control) => (
          <input
            {...control}
            data-key={key}
            className="edit"
            type="text"
            dir={key === 'name' ? 'auto' : undefined}
            inputMode={extra.numeric === true ? 'numeric' : undefined}
            autoComplete="off"
            spellCheck={false}
            disabled={extra.disabled}
            value={String(form[key])}
            onChange={(e) => {
              typeFormat(key, e.target.value as FormatForm[typeof key]);
            }}
            onBlur={() => {
              leaveFormat(key);
            }}
          />
        )}
      </FormField>
    );
  }

  function tick(key: FormatKey, label: string, hint?: string): ReactNode {
    return (
      <FormField key={key} name={key} label={label} error={errors[key]} hint={hint}>
        {(control) => (
          <input
            {...control}
            data-key={key}
            type="checkbox"
            checked={form[key] === true}
            onChange={(e) => {
              typeFormat(key, e.target.checked as FormatForm[typeof key]);
            }}
            onBlur={() => {
              leaveFormat(key);
            }}
          />
        )}
      </FormField>
    );
  }

  function choose(key: FormatKey, label: string, options: readonly [string, string][], hint?: string): ReactNode {
    return (
      <FormField key={key} name={key} label={label} error={errors[key]} hint={hint}>
        {(control) => (
          <select
            {...control}
            data-key={key}
            className="edit"
            value={String(form[key])}
            onChange={(e: ChangeEvent<HTMLSelectElement>) => {
              typeFormat(key, e.target.value as FormatForm[typeof key]);
            }}
            onBlur={() => {
              leaveFormat(key);
            }}
          >
            {options.map(([value, name]) => (
              <option key={value} value={value}>
                {name}
              </option>
            ))}
          </select>
        )}
      </FormField>
    );
  }

  const rows = [
    { marker: '', label: 'Generic', detail: 'the standard · the library folder always uses it' },
    ...stored.map((p) => ({ marker: '', label: p.name, detail: summary(p) })),
  ];

  return (
    <Dialog title="Formats" hints={mode === 'list' ? listHints : formHints} onKey={onKey}>
      {mode === 'list' ? (
        <>
          <div className="ask-body">How each player wants its files written. Choose one when syncing.</div>
          <MenuList
            label="Formats"
            rows={rows}
            cursor={cursor}
            onChoose={(i) => {
              moveFormats(i);
              if (i > 0) {
                editFormat();
              }
            }}
          />
        </>
      ) : (
        <>
          <div className="ask-body">{editing === null ? 'A new format' : 'Change this format'}</div>
          <div ref={grid} className="form">
            {text('name', 'Name')}
            <Group name="Names" />
            {tick('latin_names', 'Latin letters only', 'Files and folders are named in Latin letters')}
            {text('component_max', 'Longest name', 'Characters in a file or folder name, 20 to 255', { numeric: true })}
            {text('stem_max', 'Cut names at', 'Characters of a file name the player shows; 0 for no cut', { numeric: true })}
            <Group name="Tags" />
            {text('tag_width', 'Artist and album width', tagWidthOn(form) ? 'Characters the player shows; 0 for no limit' : 'Needs Latin letters only', {
              disabled: !tagWidthOn(form),
              numeric: true,
            })}
            {tick('latin_genre', 'Genre in Latin letters')}
            <Group name="Covers" />
            {choose('cover_format', 'Cover', [
              ['keep', 'As they are'],
              ['baseline-jpeg', 'Baseline JPEG'],
            ])}
            {text('cover_max_edge', 'Longest side', coverSizeOn(form) ? 'Pixels, 1 to 4096' : 'Only when covers are converted', {
              disabled: !coverSizeOn(form),
              numeric: true,
            })}
            {text('cover_quality', 'JPEG quality', coverSizeOn(form) ? '1 to 100' : 'Only when covers are converted', {
              disabled: !coverSizeOn(form),
              numeric: true,
            })}
            {tick('single_covers', 'Cover for each single')}
            <Group name="Playback" />
            {tick('replaygain', 'ReplayGain', 'Loudness is written so the player evens out the volume')}
            <Group name="Playlists" />
            {choose('playlist_ext', 'File type', [
              ['.m3u', '.m3u'],
              ['.m3u8', '.m3u8'],
            ])}
            {choose('playlist_style', 'Paths', [
              ['relative', 'Relative to the playlist'],
              ['root-backslash', 'From the root, with backslashes'],
            ])}
          </div>
          {error !== '' && (
            <div className="field-error" role="alert">
              <span aria-hidden="true">✗ </span>
              <bdi dir="auto">{error}</bdi>
            </div>
          )}
        </>
      )}
    </Dialog>
  );
}
