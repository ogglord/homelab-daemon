import { useState, useEffect } from "react";
import { Heading } from "@/components/ui/heading";
import { Reorder } from "motion/react";
import { WIDGETS } from "@/widgets/registry";
import { Button } from "@/components/ui/button";
import { Maximize2, Minimize2 } from "lucide-react";

export interface WidgetLayout {
  id: string;
  span: 1 | 2 | 4;
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
                // Ensure it's a valid widget and not already in layout to avoid duplicates
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
      // Default layout
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
    <div className="space-y-6 pb-12">
      <div className="flex justify-between items-center">
        <div>
          <Heading level={1}>Overview</Heading>
          <p className="text-sm text-muted-fg">System & Services</p>
        </div>
        <Button intent="secondary" size="sm" onPress={() => setIsEditing(!isEditing)}>
          {isEditing ? <Minimize2 className="w-4 h-4 mr-2" /> : <Maximize2 className="w-4 h-4 mr-2" />}
          {isEditing ? "Finish Editing" : "Edit Layout"}
        </Button>
      </div>

      <Reorder.Group 
        axis="y" 
        values={layout} 
        onReorder={setLayout} 
        className="grid grid-cols-1 md:grid-cols-4 gap-6"
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
              className={`${colSpanClass} ${isEditing ? 'cursor-grab active:cursor-grabbing hover:ring-2 hover:ring-primary rounded-xl relative z-10' : ''}`}
            >
              {isEditing && (
                <div className="absolute inset-0 z-50 bg-black/5 dark:bg-white/5 backdrop-blur-[1px] flex flex-col items-center justify-center rounded-xl transition-all">
                  <div className="bg-bg text-fg px-3 py-1.5 rounded-md shadow-sm border text-sm font-medium mb-3">
                    Drag or use buttons
                  </div>
                  <div className="flex flex-col gap-2" onPointerDown={e => e.stopPropagation()}>
                    <div className="flex gap-2 justify-center bg-bg/90 p-1.5 rounded-lg border shadow-sm">
                      <Button size="xs" intent={item.span === 1 ? "primary" : "secondary"} onPress={() => updateSpan(item.id, 1)}>1u</Button>
                      <Button size="xs" intent={item.span === 2 ? "primary" : "secondary"} onPress={() => updateSpan(item.id, 2)}>2u</Button>
                      <Button size="xs" intent={item.span === 4 ? "primary" : "secondary"} onPress={() => updateSpan(item.id, 4)}>4u</Button>
                    </div>
                    <div className="flex gap-2 justify-center bg-bg/90 p-1.5 rounded-lg border shadow-sm">
                      <Button size="xs" intent="secondary" onPress={() => moveWidget(item.id, -1)}>&larr; Move Left</Button>
                      <Button size="xs" intent="secondary" onPress={() => moveWidget(item.id, 1)}>Move Right &rarr;</Button>
                    </div>
                  </div>
                </div>
              )}
              <div className={isEditing ? 'pointer-events-none opacity-50 blur-sm transition-all h-full' : 'h-full'}>
                <Component />
              </div>
            </Reorder.Item>
          );
        })}
      </Reorder.Group>
    </div>
  );
}
