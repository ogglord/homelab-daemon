import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { RingGauge } from "@/components/ui/ring-gauge";
import { useOverview } from "@/hooks/use-overview";
import { Monitor } from "lucide-react";

export function GpuEnginesWidget() {
  const { data: stats } = useOverview((o) => o?.Stats);

  if (!stats) return <Skeleton isLoading><Card className="h-full"><CardContent>{"."}</CardContent></Card></Skeleton>;
  if (!stats.Gpu?.Available) return null;

  return (
    <Card className="h-full [--gutter:--spacing(3)]">
      <CardContent>
        {/* Header row */}
        <div className="flex items-center gap-1.5 mb-3">
          <Monitor className="h-3.5 w-3.5 text-primary" />
          <span className="text-[10px] tracking-widest uppercase text-muted-fg font-medium">iGPU Engines</span>
        </div>

        {/* Ring gauges */}
        <div className="flex items-center justify-center gap-4 flex-wrap">
          <div className="flex flex-col items-center gap-1">
            <RingGauge
              value={stats.Gpu.RenderBusy}
              max={100}
              label={`${stats.Gpu.RenderBusy.toFixed(0)}%`}
              color="var(--color-primary)"
              size={64}
              strokeWidth={5}
            />
            <span className="text-[10px] text-muted-fg uppercase tracking-wider">Render</span>
          </div>
          <div className="flex flex-col items-center gap-1">
            <RingGauge
              value={stats.Gpu.VideoBusy}
              max={100}
              label={`${stats.Gpu.VideoBusy.toFixed(0)}%`}
              color="var(--color-accent-fg)"
              size={64}
              strokeWidth={5}
            />
            <span className="text-[10px] text-muted-fg uppercase tracking-wider">Video</span>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
