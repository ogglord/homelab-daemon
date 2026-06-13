import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Sparkline } from "@/components/ui/sparkline";
import { useOverview } from "@/hooks/use-overview";
import { useSparkline } from "@/hooks/use-sparkline";
import { Cpu, Zap, Sun } from "lucide-react";

export function GpuPowerWidget() {
  const { data: stats } = useOverview((o) => o?.Stats);
  const powerHistory = useSparkline((o) => o.Stats?.Gpu?.PowerW, 30);

  if (!stats) return <Skeleton isLoading><Card className="h-full"><CardContent>{"."}</CardContent></Card></Skeleton>;
  if (!stats.Gpu?.Available) return null;

  return (
    <Card className="h-full [--gutter:--spacing(3)]">
      <CardContent>
        {/* Header row */}
        <div className="flex items-center gap-1.5 mb-2">
          <Zap className="h-3.5 w-3.5 text-warning" />
          <span className="text-[10px] tracking-[0.15em] uppercase text-primary/60 font-semibold">iGPU Power</span>
        </div>

        {/* Stat rows */}
        <div className="space-y-1.5 mb-2">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-1.5">
              <Cpu className="h-3 w-3 text-primary" />
              <span className="text-xs text-muted-fg">Freq</span>
            </div>
            <span className="text-xs font-mono font-bold text-fg">{stats.Gpu.FreqMHz.toFixed(0)} MHz</span>
          </div>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-1.5">
              <Zap className="h-3 w-3 text-warning" />
              <span className="text-xs text-muted-fg">Power</span>
            </div>
            <span className="text-xs font-mono font-bold text-fg">{stats.Gpu.PowerW.toFixed(1)} W</span>
          </div>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-1.5">
              <Sun className="h-3 w-3 text-success" />
              <span className="text-xs text-muted-fg">RC6 idle</span>
            </div>
            <span className="text-xs font-mono font-bold text-fg">{stats.Gpu.RC6Pct.toFixed(0)}%</span>
          </div>
        </div>

        {/* Mini sparkline */}
        <Sparkline
          data={powerHistory}
          width={200}
          height={24}
          color="var(--color-warning)"
          filled
          className="w-full"
        />
      </CardContent>
    </Card>
  );
}
