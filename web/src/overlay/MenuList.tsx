import { useLayoutEffect, useRef } from 'react';

export interface MenuRow {
  marker: string;
  label: string;
  detail: string;
  off?: boolean;
}

interface MenuListProps {
  label: string;
  rows: readonly MenuRow[];
  cursor: number;
  onChoose: (i: number) => void;
}

export function MenuList({ label, rows, cursor, onChoose }: MenuListProps) {
  const ref = useRef<HTMLDivElement>(null);

  useLayoutEffect(() => {
    ref.current?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: 'nearest' });
  }, [cursor]);

  return (
    <div ref={ref} className="menu" role="listbox" aria-label={label}>
      {rows.map((row, i) => (
        <div
          key={i}
          role="option"
          aria-selected={i === cursor}
          aria-disabled={row.off === true ? true : undefined}
          className={['item', i === cursor ? 'on' : '', row.off === true ? 'off' : ''].join(' ').trim()}
          onClick={() => {
            onChoose(i);
          }}
        >
          <span className="marker">{row.marker}</span>
          <span className="label">
            <bdi dir="auto">{row.label}</bdi>
          </span>
          <span className="detail">
            <bdi dir="auto">{row.detail}</bdi>
          </span>
        </div>
      ))}
    </div>
  );
}

export function moveIn(length: number, at: number, by: number): number {
  return Math.min(Math.max(at + by, 0), length - 1);
}
