export function Problem({ text }: { text: string }) {
  return (
    <div className="bad" role="alert">
      <bdi dir="auto">{text}</bdi>
    </div>
  );
}
