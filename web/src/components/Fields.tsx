import { Fragment, useEffect, useRef, type KeyboardEvent } from 'react';
import type { BarView, FieldView } from '../api/types.gen';
import { cancelEdit, chooseField, saveEdit, typeEdit } from '../state/edit';
import { fieldIndex, nudgeable } from '../state/inspector';
import { useApp, type EditState } from '../state/store';

interface EditorProps {
  field: FieldView;
  edit: EditState;
  second: boolean;
}

function Editor({ field, edit, second }: EditorProps) {
  const first = useRef<HTMLInputElement>(null);
  const other = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const input = first.current;
    if (input !== null) {
      input.focus({ preventScroll: true });
      input.setSelectionRange(input.value.length, input.value.length);
    }
  }, []);

  function keys(e: KeyboardEvent<HTMLInputElement>): void {
    if (e.nativeEvent.isComposing) {
      return;
    }
    if (e.key === 'Enter') {
      e.preventDefault();
      void saveEdit();
    } else if (e.key === 'Escape') {
      e.preventDefault();
      cancelEdit();
    } else if (e.key === 'Tab' && second) {
      e.preventDefault();
      (e.currentTarget === first.current ? other.current : first.current)?.focus();
    }
  }

  const text = (
    <input
      ref={first}
      className="edit"
      type="text"
      dir="auto"
      autoComplete="off"
      spellCheck={false}
      aria-label={field.label}
      placeholder={field.mixed === true ? '(mixed)' : undefined}
      value={edit.text}
      onChange={(e) => {
        typeEdit('text', e.target.value);
      }}
      onKeyDown={keys}
    />
  );
  if (!second) {
    return text;
  }
  return (
    <span className="pair">
      {text}
      <span className="dim">of</span>
      <input
        ref={other}
        className="edit short"
        type="text"
        inputMode="numeric"
        autoComplete="off"
        aria-label="Tracks on album"
        value={edit.second ?? ''}
        onChange={(e) => {
          typeEdit('second', e.target.value);
        }}
        onKeyDown={keys}
      />
    </span>
  );
}

function KeptBar({ bar }: { bar: BarView }) {
  const total = Math.max(bar.total, 1);
  const share = (n: number) => `0 0 ${String((100 * n) / total)}%`;
  const kept = Math.max(total - bar.start - bar.end, 0);
  return (
    <div className="value">
      <div className="bar" role="img" aria-label={bar.kept}>
        <i className="cut" style={{ flex: share(bar.start) }} />
        <i className="kept" style={{ flex: share(kept) }} />
        <i className="cut" style={{ flex: share(bar.end) }} />
      </div>
      <div className="hintline mono">{bar.kept}</div>
    </div>
  );
}

function LevelBar({ level }: { level: readonly number[] }) {
  const path = level
    .map((v, i) => {
      if (v < 0) {
        return '';
      }
      const h = Math.max(v * 10, 0.3);
      return `M${String(i)} ${String(10 - h)}h1v${String(h)}h-1z`;
    })
    .join('');
  return (
    <svg className="level" viewBox="0 0 100 10" preserveAspectRatio="none" role="img" aria-label="Volume across the track, with the fades">
      <path d={path} />
    </svg>
  );
}

interface FieldsProps {
  fields: readonly FieldView[];
  bar?: BarView | undefined;
  numbers: boolean;
}

export function Fields({ fields, bar, numbers }: FieldsProps) {
  const focus = useApp((s) => s.focus);
  const index = useApp(fieldIndex);
  const edit = useApp((s) => s.edit);

  return (
    <div className="fields">
      {fields.map((f, i) => {
        const cur = focus === 'inspector' && i === index;
        const editing = edit !== null && edit.field === i;
        const dim = f.dim === true || f.text === '' || !f.editable;
        const classes = ['value', dim ? 'dim' : '', cur ? 'cur' : '', nudgeable.includes(f.key) ? 'mono' : ''].filter((c) => c !== '');
        const choose = () => {
          chooseField(i);
        };
        return (
          <Fragment key={f.key}>
            <div className={cur ? 'label cur' : 'label'} onClick={choose}>
              {f.label}
            </div>
            {editing ? (
              <div className="value cur editing" data-testid={`value-${f.key}`}>
                <Editor field={f} edit={edit} second={numbers && f.key === 'track'} />
              </div>
            ) : (
              <div className={classes.join(' ')} data-testid={`value-${f.key}`} onClick={choose}>
                {f.text === '' ? (f.empty ?? '') : <bdi dir="auto">{f.text}</bdi>}
              </div>
            )}
            {cur && f.hint !== undefined && f.hint !== '' && <div className="span hintline">{f.hint}</div>}
            {cur && editing && edit.note !== undefined && edit.note !== '' && (
              <div className="span note-live">
                <bdi dir="auto">{edit.note}</bdi>
              </div>
            )}
            {cur && editing && edit.error !== undefined && edit.error !== '' && (
              <div className="span bad" role="alert">
                {edit.error}
              </div>
            )}
            {f.key === 'cutEnd' && bar !== undefined && (
              <>
                <div className="label">Kept</div>
                <KeptBar bar={bar} />
              </>
            )}
            {f.key === 'fadeOut' && bar?.level !== undefined && bar.level.length > 0 && (
              <>
                <div className="label">Level</div>
                <div className="value">
                  <LevelBar level={bar.level} />
                </div>
              </>
            )}
          </Fragment>
        );
      })}
    </div>
  );
}
