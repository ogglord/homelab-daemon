import { useState, useEffect, useRef } from "react";
import { Reorder } from "motion/react";
import { WIDGETS } from "@/widgets/registry";
import { Button } from "@/components/ui/button";
import { useOverview } from "@/hooks/use-overview";
import { LayoutGrid, GripVertical, ChevronLeft, ChevronRight } from "lucide-react";

export interface WidgetLayout {
  id: string;
  span: 1 | 2 | 4;
}

function PollChip() {
  const lastFetchRef = useRef(Date.now());
  const [ago, setAgo] = useState(0);

  // Re-render the "Ns ago" text every second
  useEffect(() => {
    const timer = setInterval(() => {
      setAgo(Math.round((Date.now() - lastFetchRef.current) / 1000));
    }, 1000);
    return () => clearInterval(timer);
  }, []);

  // On every overview tick, reset the timestamp
  const { data } = useOverview();
  useEffect(() => {
    if (data) lastFetchRef.current = Date.now();
  }, [data]);

  return (
    <div className="flex items-center gap-1.5 text-[10px] font-mono text-muted-fg">
      <span className="inline-block size-1.5 rounded-full bg-success animate-pulse" />
      <span>{ago}s ago</span>
    </div>
  );
}

export default function OverviewPage() {
  const [layout, setLayout] = useState<WidgetLayout[]>(() => {
    const saved = localStorage.getItem("dash-overview-layout");

    let currentLayout: WidgetLayout[] = [];

    if (saved) {
      try {
        const parsed = JSON.parse(saved);
        if (Array.isArray(parsed) && parsed.length > 0) {
          if (typeof parsed[0] === "string") {
            // Migrate from string[] to WidgetLayout[]
            parsed.forEach(id => {
              if (id === 'cpu-ram') {
                currentLayout.push({ id: 'cpu', span: 2 });
                currentLayout.push({ id: 'memory', span: 2 });
              } else if (id === 'temperature') {
                currentLayout.push({ id: 'temperature', span: 1 });
                currentLayout.push({ id: 'uptime', span: 1 });
              } else if (id === 'gpu') {
                currentLayout.push({ id: 'gpu-engines', span: 1 });
                currentLayout.push({ id: 'gpu-power', span: 1 });
              } else if (id === 'processes') {
                currentLayout.push({ id: 'top-cpu', span: 2 });
                currentLayout.push({ id: 'top-memory', span: 2 });
              } else {
                const w = WIDGETS.find(w => w.id === id);
                if (w) currentLayout.push({ id, span: (w.defaultGridSpan === 2 ? 4 : 2) as 1|2|4 });
              }
            });
          } else {
            // Already WidgetLayout[] format, but might contain old IDs
            parsed.forEach((item: any) => {
              if (item.id === 'cpu-ram') {
                currentLayout.push({ id: 'cpu', span: item.span === 4 ? 2 : 1 });
                currentLayout.push({ id: 'memory', span: item.span === 4 ? 2 : 1 });
              } else if (item.id === 'gpu') {
                currentLayout.push({ id: 'gpu-engines', span: 1 });
                currentLayout.push({ id: 'gpu-power', span: 1 });
              } else if (item.id === 'processes') {
                currentLayout.push({ id: 'top-cpu', span: item.span === 4 ? 2 : 1 });
                currentLayout.push({ id: 'top-memory', span: item.span === 4 ? 2 : 1 });
              } else {
                const w = WIDGETS.find(w => w.id === item.id);
                if (w && !currentLayout.some(existing => existing.id === item.id)) {
                  currentLayout.push(item);
                }
              }
            });
          }
        }
      } catch (e) {}
    }

    if (currentLayout.length === 0) {
      currentLayout = WIDGETS.map((w) => ({
        id: w.id,
        span: (w.defaultGridSpan === 2 ? 4 : 2) as 1 | 2 | 4
      }));
    } else {
      // Append any new widgets that aren't in the saved layout
      const existingIds = currentLayout.map(item => item.id);
      const newWidgets = WIDGETS.filter(w => !existingIds.includes(w.id)).map(w => ({
        id: w.id,
        span: (w.defaultGridSpan === 2 ? 4 : 2) as 1 | 2 | 4
      }));
      currentLayout = [...currentLayout, ...newWidgets];
    }

    return currentLayout;
  });

  const [isEditing, setIsEditing] = useState(false);

  useEffect(() => {
    localStorage.setItem("dash-overview-layout", JSON.stringify(layout));
  }, [layout]);

  const updateSpan = (id: string, span: 1 | 2 | 4) => {
    setLayout(prev => prev.map(item => item.id === id ? { ...item, span } : item));
  };

  const moveWidget = (id: string, direction: -1 | 1) => {
    setLayout(prev => {
      const idx = prev.findIndex(item => item.id === id);
      if (idx < 0) return prev;
      const newIdx = idx + direction;
      if (newIdx < 0 || newIdx >= prev.length) return prev;

      const newLayout = [...prev];
      const temp = newLayout[idx];
      newLayout[idx] = newLayout[newIdx];
      newLayout[newIdx] = temp;
      return newLayout;
    });
  };

  return (
    <div className="space-y-3 pb-12">
      {/* Compact header */}
      <div className="flex justify-between items-center">
        <div className="flex items-center gap-3">
          <span className="text-[10px] tracking-widest uppercase text-muted-fg font-mono">Overview</span>
          <PollChip />
        </div>
        <Button
          intent="plain"
          size="sq-xs"
          onPress={() => setIsEditing(!isEditing)}
          aria-label={isEditing ? "Finish editing" : "Edit layout"}
        >
          <LayoutGrid className={`w-4 h-4 ${isEditing ? "text-primary" : ""}`} />
        </Button>
      </div>

      <Reorder.Group
        axis="y"
        values={layout}
        onReorder={setLayout}
        className="grid grid-cols-1 md:grid-cols-4 gap-2"
      >
        {layout.map((item) => {
          const widget = WIDGETS.find((w) => w.id === item.id);
          if (!widget) return null;
          const Component = widget.component;

          const colSpanClass = item.span === 4 ? 'md:col-span-4' : item.span === 2 ? 'md:col-span-2' : 'md:col-span-1';

          return (
            <Reorder.Item
              key={item.id}
              value={item}
              dragListener={isEditing}
              className={`${colSpanClass} ${isEditing ? 'cursor-grab active:cursor-grabbing ring-1 ring-border rounded-lg relative' : ''}`}
            >
              {/* Edit toolbar — compact, visible above the content */}
              {isEditing && (
                <div className="flex items-center justify-between px-2 py-1 bg-muted/50 rounded-t-lg border-b border-border">
                  <div className="flex items-center gap-1">
                    <GripVertical className="h-3 w-3 text-muted-fg" />
                    <span className="text-[10px] text-muted-fg font-mono">{widget.name}</span>
                  </div>
                  <div className="flex items-center gap-0.5">
                    <Button size="sq-xs" intent="plain" onPress={() => moveWidget(item.id, -1)}>
                      <ChevronLeft className="h-3 w-3" />
                    </Button>
                    <Button size="sq-xs" intent="plain" onPress={() => moveWidget(item.id, 1)}>
                      <ChevronRight className="h-3 w-3" />
                    </Button>
                    <div className="flex gap-px ml-1">
                      {([1, 2, 4] as const).map((s) => (
                        <button
                          key={s}
                          onClick={() => updateSpan(item.id, s)}
                          className={`text-[9px] font-mono px-1.5 py-0.5 rounded ${item.span === s ? 'bg-primary text-primary-fg' : 'bg-secondary text-secondary-fg hover:bg-accent'}`}
                        >
                          {s}u
                        </button>
                      ))}
                    </div>
                  </div>
                </div>
              )}
              <div className="h-full">
                <Component />
              </div>
            </Reorder.Item>
          );
        })}
      </Reorder.Group>
    </div>
  );
}
