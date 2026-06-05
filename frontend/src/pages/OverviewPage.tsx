import { useState, useEffect, useRef } from "react";
import { WIDGETS } from "@/widgets/registry";
import type { WidgetLayout } from "@/widgets/registry";

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

import { useOverview } from "@/hooks/use-overview";

export default function OverviewPage() {
  const [layout, setLayout] = useState<WidgetLayout[] | null>(null);

  useEffect(() => {
    fetch("/widgets.json")
      .then((r) => r.json())
      .then((data) => {
        if (Array.isArray(data)) setLayout(data);
      })
      .catch(() => {
        // Fallback: use widget registry order
        setLayout(
          WIDGETS.map((w) => ({
            id: w.id,
            span: (w.defaultGridSpan ?? 1) as 1 | 2 | 4,
          })),
        );
      });
  }, []);

  return (
    <div className="space-y-3 pb-12">
      <div className="flex justify-between items-center">
        <div className="flex items-center gap-3">
          <span className="text-[10px] tracking-widest uppercase text-muted-fg font-mono">Overview</span>
          <PollChip />
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-2">
        {(layout ?? [])
          .filter((item) => WIDGETS.some((w) => w.id === item.id))
          .map((item) => {
            const widget = WIDGETS.find((w) => w.id === item.id)!;
            const Component = widget.component;
            const colSpanClass =
              item.span === 4
                ? "md:col-span-4"
                : item.span === 2
                  ? "md:col-span-2"
                  : "md:col-span-1";
            return (
              <div key={item.id} className={colSpanClass}>
                <Component />
              </div>
            );
          })}
      </div>
    </div>
  );
}
