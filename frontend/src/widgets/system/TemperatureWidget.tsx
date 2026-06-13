import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { RingGauge } from "@/components/ui/ring-gauge";
import { useOverview } from "@/hooks/use-overview";
import { Thermometer } from "lucide-react";

export function TemperatureWidget() {
  const { data: stats } = useOverview((o) => o?.Stats);

  if (!stats) return <Skeleton isLoading><Card className="h-full"><CardContent>{"."}</CardContent></Card></Skeleton>;

  const cpuColor = stats.CPUTemp > 80 ? "var(--color-danger)" : stats.CPUTemp > 60 ? "var(--color-warning)" : "var(--color-success)";
  const nvmeColor = stats.NVMeTemp > 70 ? "var(--color-danger)" : stats.NVMeTemp > 55 ? "var(--color-warning)" : "var(--color-primary)";

  return (
    <Card className="h-full [--gutter:--spacing(3)]">
      <CardContent>
        {/* Header row */}
        <div className="flex items-center gap-1.5 mb-3">
          <Thermometer className="h-3.5 w-3.5 text-warning" />
          <span className="text-[10px] tracking-[0.15em] uppercase text-primary/60 font-semibold">Temperatures</span>
        </div>

        {/* Ring gauges */}
        <div className="flex items-center justify-center gap-4 flex-wrap">
          <div className="flex flex-col items-center gap-1">
            <RingGauge
              value={stats.CPUTemp}
              max={100}
              label={`${stats.CPUTemp.toFixed(0)}°`}
              color={cpuColor}
              size={64}
              strokeWidth={5}
            />
            <span className="text-[10px] text-muted-fg uppercase tracking-wider">CPU</span>
          </div>
          <div className="flex flex-col items-center gap-1">
            <RingGauge
              value={stats.NVMeTemp}
              max={100}
              label={`${stats.NVMeTemp.toFixed(0)}°`}
              color={nvmeColor}
              size={64}
              strokeWidth={5}
            />
            <span className="text-[10px] text-muted-fg uppercase tracking-wider">NVMe</span>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
