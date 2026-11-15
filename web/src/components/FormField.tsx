import { useId, type ReactNode } from 'react';

export interface ControlProps {
  id: string;
  'aria-invalid': true | undefined;
  'aria-describedby': string | undefined;
}

interface FormFieldProps {
  name: string;
  label: string;
  error?: string | undefined;
  hint?: string | undefined;
  hideLabel?: boolean;
  children: (control: ControlProps) => ReactNode;
}

export function FormField({ name, label, error, hint, hideLabel = false, children }: FormFieldProps) {
  const base = useId();
  const id = `${base}-${name}`;
  const hintId = `${id}-hint`;
  const errorId = `${id}-error`;
  const wrong = error !== undefined && error !== '';
  const described = [hint === undefined || hint === '' ? '' : hintId, wrong ? errorId : ''].filter((s) => s !== '').join(' ');

  return (
    <div className={['field', wrong ? 'wrong' : '', hideLabel ? 'bare' : ''].filter((c) => c !== '').join(' ')}>
      <label className={hideLabel ? 'sr-only' : 'label'} htmlFor={id}>
        {label}
      </label>
      <div className="control">
        {children({ id, 'aria-invalid': wrong ? true : undefined, 'aria-describedby': described === '' ? undefined : described })}
        {hint !== undefined && hint !== '' && (
          <div id={hintId} className="hintline">
            <bdi dir="auto">{hint}</bdi>
          </div>
        )}
        {wrong && (
          <div id={errorId} className="field-error" role="alert">
            <span aria-hidden="true">✗ </span>
            <bdi dir="auto">{error}</bdi>
          </div>
        )}
      </div>
    </div>
  );
}
