import { useEffect, useRef } from 'react';
import type { KeyEventLike } from '../keys/keymap';
import { FormField } from '../components/FormField';
import { Dialog } from '../overlay/Dialog';
import { ringKey } from '../overlay/focusRing';
import { applyBulk, bulkFields, leaveBulk, typeBulk, useBulk } from '../state/bulk';
import { shownErrors } from '../state/form';
import { plural } from '../state/text';

interface BulkFormProps {
  ids: readonly string[];
  head: string;
}

const hints = [
  { keys: 'enter', label: 'apply' },
  { keys: 'tab', label: 'next field' },
  { keys: 'esc', label: 'close' },
];

export function BulkForm({ ids, head }: BulkFormProps) {
  const values = useBulk((s) => s.values);
  const summary = useBulk((s) => s.summary);
  const checks = useBulk((s) => s.checks);
  const errors = shownErrors(checks);
  const inputs = useRef<(HTMLInputElement | null)[]>([]);

  useEffect(() => {
    inputs.current[0]?.focus();
  }, []);

  function apply(): void {
    void applyBulk().then((wrong) => {
      inputs.current[bulkFields.findIndex((f) => f.key === wrong)]?.focus();
    });
  }

  function onKey(e: KeyEventLike): boolean {
    if (e.key === 'Enter') {
      apply();
      return true;
    }
    return ringKey(
      e,
      inputs.current.filter((input) => input !== null),
    );
  }

  return (
    <Dialog title={head} hints={hints} onKey={onKey}>
      <div className="ask-body">{`Applies to ${plural(ids.length, 'track')}. Blank keeps what they have; - clears.`}</div>
      <div className="form">
        {bulkFields.map((f, i) => (
          <FormField key={f.key} name={f.key} label={f.label} error={errors[f.key]}>
            {(control) => (
              <input
                {...control}
                ref={(input) => {
                  inputs.current[i] = input;
                }}
                className="edit"
                type="text"
                dir="auto"
                autoComplete="off"
                spellCheck={false}
                placeholder="(keep)"
                value={values[f.key]}
                onChange={(e) => {
                  typeBulk(f.key, e.target.value);
                }}
                onBlur={() => {
                  leaveBulk(f.key);
                }}
              />
            )}
          </FormField>
        ))}
      </div>
      <div className={summary.bad ? 'bulk-summary bad' : 'bulk-summary'} data-testid="bulk-summary" aria-live="polite">
        <bdi dir="auto">{summary.text}</bdi>
      </div>
    </Dialog>
  );
}
