import { useState, useEffect, useRef, useMemo } from "react";
import { WIDGETS } from "@/widgets/registry";
import type { WidgetLayout } from "@/widgets/registry";
import { useOverview } from "@/hooks/use-overview";

type Row = { items: WidgetLayout[]; totalSpan: number };

function computeRows(layout: WidgetLayout[], cols: number): Row[] {
  const rows: Row[] = [];
  let current: WidgetLayout[] = [];
  let used = 0;

  for (const item of layout) {
    const span = Math.min(item.span, cols);
    if (used + span > cols) {
      rows.push({ items: current, totalSpan: used });
      current = [];
      used = 0;
    }
    current.push({ ...item, span: span as 1 | 2 | 4 });
    used += span;
  }
  if (current.length > 0) rows.push({ items: current, totalSpan: used });

  // Distribute leftover capacity to the last widget in each row.
  for (const row of rows) {
    if (row.totalSpan < cols && row.items.length > 0) {
      const last = row.items[row.items.length - 1];
      last.span = Math.min(last.span + (cols - row.totalSpan), cols) as 1 | 2 | 4;
    }
  }

  return rows;
}

function PollChip() {
  const lastFetchRef = useRef(Date.now());
  const { data } = useOverview();
  const [ago, setAgo] = useState(0);

  useEffect(() => {
    if (data) lastFetchRef.current = Date.now();
  }, [data]);

  useEffect(() => {
    const timer = setInterval(() => {
      setAgo(Math.round((Date.now() - lastFetchRef.current) / 1000));
    }, 1000);
    return () => clearInterval(timer);
  }, []);

  return (
    <div className="flex items-center gap-1.5 text-[10px] font-mono text-muted-fg">
      <span className="inline-block size-1.5 rounded-full bg-success animate-pulse" />
      <span>{ago}s ago</span>
    </div>
  );
}

export default function OverviewPage() {
  const [layout, setLayout] = useState<WidgetLayout[] | null>(null);

  useEffect(() => {
    fetch("/widgets.json")
      .then((r) => r.json())
      .then((data) => {
        if (Array.isArray(data)) setLayout(data);
      })
      .catch(() => {
        setLayout(
          WIDGETS.map((w) => ({
            id: w.id,
            span: (w.defaultGridSpan ?? 1) as 1 | 2 | 4,
          })),
        );
      });
  }, []);

  const rows = useMemo(() => {
    const active = (layout ?? [])
      .filter((item) => WIDGETS.some((w) => w.id === item.id));
    return computeRows(active, 4);
  }, [layout]);

  /** Responsive column-span: mobile always 1, desktop respects item.span */
  function spanClass(span: number) {
    if (span === 4) return 'col-span-1 md:col-span-4';
    if (span === 2) return 'col-span-1 md:col-span-2';
    return 'col-span-1 md:col-span-1';
  }

  return (
    <div className="space-y-2 pb-12">
      <div className="flex justify-between items-center">
        <div className="flex items-center gap-3">
          <span className="text-[10px] tracking-widest uppercase text-muted-fg font-mono">Overview</span>
          <PollChip />
        </div>
      </div>

      <div className="flex flex-col gap-2">
        {rows.map((row, ri) => (
          <div key={ri} className="grid grid-cols-1 md:grid-cols-4 gap-2 auto-rows-auto">
            {row.items.map((item) => {
              const widget = WIDGETS.find((w) => w.id === item.id);
              if (!widget) return null;
              const Component = widget.component;
              return (
                <div key={item.id} className={`min-w-0 ${spanClass(item.span)}`}>
                  <div className="h-full">
                    <Component />
                  </div>
                </div>
              );
            })}
          </div>
        ))}
      </div>
    </div>
  );
}
