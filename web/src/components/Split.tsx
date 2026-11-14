import { useRef, useState, type CSSProperties, type PointerEvent, type ReactNode } from 'react';
import { recall, remember, useApp } from '../state/store';

interface SplitProps {
  leftTitle: string;
  rightTitle: string;
  left: ReactNode;
  right: ReactNode;
}

const leftKey = 'mm.left';
const minLeft = 220;
const minRight = 300;

function recallLeft(): number | null {
  const px = Number(recall(leftKey));
  return Number.isFinite(px) && px > 0 ? px : null;
}

export function Split({ leftTitle, rightTitle, left, right }: SplitProps) {
  const focus = useApp((s) => s.focus);
  const dialogOpen = useApp((s) => s.overlay !== null);
  const [width, setWidth] = useState(recallLeft);
  const leftPane = useRef<HTMLElement>(null);
  const rightPane = useRef<HTMLElement>(null);
  const divider = useRef<HTMLDivElement>(null);

  function widthAt(e: PointerEvent): number | null {
    const first = leftPane.current?.getBoundingClientRect();
    const last = rightPane.current?.getBoundingClientRect();
    const bar = divider.current?.getBoundingClientRect();
    if (!first || !last || !bar) {
      return null;
    }
    const room = last.right - first.left - bar.width - minRight;
    return Math.round(Math.min(Math.max(e.clientX - first.left - bar.width / 2, minLeft), Math.max(room, minLeft)));
  }

  function drag(e: PointerEvent<HTMLDivElement>): void {
    if (!e.currentTarget.hasPointerCapture(e.pointerId)) {
      return;
    }
    const px = widthAt(e);
    if (px !== null) {
      setWidth(px);
    }
  }

  function drop(e: PointerEvent<HTMLDivElement>): void {
    const px = widthAt(e);
    if (px !== null) {
      setWidth(px);
      remember(leftKey, String(px));
    }
  }

  const style = {
    '--left-min': `${String(minLeft)}px`,
    '--right-min': `${String(minRight)}px`,
    ...(width === null ? {} : { '--left': `${String(width)}px` }),
  } as CSSProperties;

  return (
    <main className="panes" data-show={focus} style={style}>
      <section ref={leftPane} className={focus === 'tree' && !dialogOpen ? 'pane pane-tree focus' : 'pane pane-tree'} aria-label="Library">
        <h2 className="pane-title">{leftTitle}</h2>
        {left}
      </section>
      <div
        ref={divider}
        className="divider"
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize the panes"
        onPointerDown={(e) => {
          e.currentTarget.setPointerCapture(e.pointerId);
        }}
        onPointerMove={drag}
        onPointerUp={drop}
      />
      <section
        ref={rightPane}
        className={focus === 'inspector' && !dialogOpen ? 'pane pane-inspector focus' : 'pane pane-inspector'}
        aria-label="Details"
      >
        <h2 className="pane-title">{rightTitle}</h2>
        {right}
      </section>
    </main>
  );
}
