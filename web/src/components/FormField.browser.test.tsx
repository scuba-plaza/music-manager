import { describe, expect, test } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-react';
import { FormField } from './FormField';

function Titled({ error, hint }: { error?: string; hint?: string }) {
  return (
    <div className="form">
      <FormField name="title" label="Title" error={error} hint={hint}>
        {(control) => <input className="edit" type="text" {...control} readOnly value="So What" />}
      </FormField>
    </div>
  );
}

describe('FormField', () => {
  test('names its input by its label', async () => {
    await render(<Titled />);

    await expect.element(page.getByRole('textbox', { name: 'Title' })).toBeVisible();
  });

  test('has no alert and no invalid mark when nothing is wrong', async () => {
    await render(<Titled />);

    await expect.element(page.getByRole('alert')).not.toBeInTheDocument();
    await expect.element(page.getByRole('textbox', { name: 'Title' })).not.toHaveAttribute('aria-invalid');
  });

  test('says what is wrong under the input and marks the input invalid', async () => {
    await render(<Titled error="Title cannot be empty" />);

    const input = page.getByRole('textbox', { name: 'Title' });
    await expect.element(input).toBeInvalid();
    await expect.element(page.getByRole('alert')).toHaveTextContent('Title cannot be empty');
    const alert = page.getByRole('alert').element();
    expect(input.element().getAttribute('aria-describedby')).toContain(alert.id);
  });

  test('shows a hint that the input is described by, and keeps it when something is wrong', async () => {
    const view = await render(<Titled hint="e.g. 1999-05-17" />);

    const input = page.getByRole('textbox', { name: 'Title' });
    await expect.element(page.getByText('e.g. 1999-05-17')).toBeVisible();
    const hint = page.getByText('e.g. 1999-05-17').element();
    expect(input.element().getAttribute('aria-describedby')).toContain(hint.id);

    await view.rerender(<Titled hint="e.g. 1999-05-17" error="Not a date" />);

    await expect.element(page.getByText('e.g. 1999-05-17')).toBeVisible();
    await expect.element(page.getByRole('alert')).toHaveTextContent('Not a date');
    const describedBy = input.element().getAttribute('aria-describedby') ?? '';
    expect(describedBy).toContain(hint.id);
    expect(describedBy).toContain(page.getByRole('alert').element().id);
  });

  test('can keep its label out of sight and still name the input by it', async () => {
    await render(
      <div className="form">
        <FormField name="link" label="Link" hideLabel>
          {(control) => <input className="edit" type="text" {...control} readOnly value="" />}
        </FormField>
      </div>,
    );

    await expect.element(page.getByRole('textbox', { name: 'Link' })).toBeVisible();
    expect(document.querySelector('label.label')).toBeNull();
  });

  test('gives two fields on a page ids of their own', async () => {
    await render(
      <div className="form">
        <FormField name="title" label="Title" error="One">
          {(control) => <input className="edit" type="text" {...control} readOnly value="" />}
        </FormField>
        <FormField name="artist" label="Artist" error="Two">
          {(control) => <input className="edit" type="text" {...control} readOnly value="" />}
        </FormField>
      </div>,
    );

    const ids = page
      .getByRole('textbox')
      .elements()
      .map((e) => e.id);
    expect(new Set(ids).size).toBe(2);
    const alerts = page
      .getByRole('alert')
      .elements()
      .map((e) => e.id);
    expect(new Set(alerts).size).toBe(2);
  });
});
