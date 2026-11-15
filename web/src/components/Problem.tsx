interface ProblemProps {
  text: string;
  kind?: 'bad' | 'warn';
}

export function Problem({ text, kind = 'bad' }: ProblemProps) {
  return (
    <div className={kind} role="alert">
      <bdi dir="auto">{text}</bdi>
    </div>
  );
}
