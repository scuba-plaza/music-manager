import { useId, type ReactNode } from 'react';

interface ControlProps {
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

interface FieldErrorProps {
  text: string;
  id?: string | undefined;
  span?: boolean;
}

export function FieldError({ text, id, span = false }: FieldErrorProps) {
  return (
    <div id={id} className={span ? 'span field-error' : 'field-error'} role="alert">
      <span aria-hidden="true">✗ </span>
      <bdi dir="auto">{text}</bdi>
    </div>
  );
}

export function FormField({ name, label, error = '', hint = '', hideLabel = false, children }: FormFieldProps) {
  const base = useId();
  const id = `${base}-${name}`;
  const hintId = `${id}-hint`;
  const errorId = `${id}-error`;
  const wrong = error !== '';
  const described = [hint === '' ? '' : hintId, wrong ? errorId : ''].filter((s) => s !== '').join(' ');

  return (
    <div className={['field', wrong ? 'wrong' : '', hideLabel ? 'bare' : ''].filter((c) => c !== '').join(' ')}>
      <label className={hideLabel ? 'sr-only' : 'label'} htmlFor={id}>
        {label}
      </label>
      <div className="control">
        {children({ id, 'aria-invalid': wrong ? true : undefined, 'aria-describedby': described === '' ? undefined : described })}
        {hint !== '' && (
          <div id={hintId} className="hintline">
            <bdi dir="auto">{hint}</bdi>
          </div>
        )}
        {wrong && <FieldError id={errorId} text={error} />}
      </div>
    </div>
  );
}
