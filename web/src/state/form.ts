export interface FormChecks {
  readonly wrong: Readonly<Record<string, string>>;
  readonly left: Readonly<Record<string, true>>;
  readonly tried: boolean;
}

export function cleanChecks(): FormChecks {
  return { wrong: {}, left: {}, tried: false };
}

export function verdict(checks: FormChecks, wrong: Readonly<Record<string, string>>): FormChecks {
  return { ...checks, wrong };
}

export function left(checks: FormChecks, key: string): FormChecks {
  return { ...checks, left: { ...checks.left, [key]: true } };
}

export function tried(checks: FormChecks): FormChecks {
  return { ...checks, tried: true };
}

export function shownErrors(checks: FormChecks): Record<string, string> {
  const shown: Record<string, string> = {};
  for (const [key, text] of Object.entries(checks.wrong)) {
    if (checks.tried || key in checks.left) {
      shown[key] = text;
    }
  }
  return shown;
}

export function firstWrong<K extends string>(checks: FormChecks, order: readonly K[]): K | undefined {
  const shown = shownErrors(checks);
  return order.find((key) => key in shown);
}
