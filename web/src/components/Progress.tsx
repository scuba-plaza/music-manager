interface ProgressProps {
  done: number;
  total: number;
  label: string;
}

export function Progress({ done, total, label }: ProgressProps) {
  const percent = total > 0 ? Math.round((100 * done) / total) : 0;
  return (
    <div className="progress-row">
      <div className="progress" role="progressbar" aria-label={label} aria-valuemin={0} aria-valuemax={total} aria-valuenow={done}>
        <i style={{ inlineSize: `${String(percent)}%` }} />
      </div>
      <span className="dim">{`${String(done)}/${String(total)}`}</span>
    </div>
  );
}
