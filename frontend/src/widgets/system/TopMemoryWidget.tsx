import { Card, CardContent } from "@/components/ui/card";
import { Meter, MeterTrack } from "@/components/ui/meter";
import { Skeleton } from "@/components/ui/skeleton";
import { useOverview } from "@/hooks/use-overview";
import type { ProcessStat } from "@/types";
import { MemoryStick } from "lucide-react";

function ProcessRows({ processes, valueKey, valueSuffix, color }: {
  processes: ProcessStat[];
  valueKey: "CPU" | "Memory";
  valueSuffix: string;
  color: string;
}) {
  return (
    <div className="space-y-1.5">
      {processes.slice(0, 5).map((p) => {
        const val = p[valueKey];
        const pct = Math.min(val, 100);
        return (
          <div key={p.PID}>
            <div className="flex justify-between text-xs mb-0.5">
              <span className="text-fg truncate pr-2">{p.Name}</span>
              <span className="font-mono text-muted-fg shrink-0">{val.toFixed(1)}{valueSuffix}</span>
            </div>
            <Meter value={pct} color={color}>
              <MeterTrack className="[--meter-height:--spacing(1)]" />
            </Meter>
          </div>
        );
      })}
    </div>
  );
}

export function TopMemoryWidget() {
  const { data: stats } = useOverview((o) => o?.Stats);

  if (!stats) return <Skeleton isLoading><Card className="h-full"><CardContent>{"."}</CardContent></Card></Skeleton>;

  return (
    <Card className="h-full [--gutter:--spacing(3)]">
      <CardContent>
        {/* Header row */}
        <div className="flex items-center gap-1.5 mb-2">
          <MemoryStick className="h-3.5 w-3.5 text-success" />
          <span className="text-[10px] tracking-widest uppercase text-muted-fg font-medium">Top Memory</span>
        </div>

        {stats.TopMem?.length ? (
          <ProcessRows processes={stats.TopMem} valueKey="Memory" valueSuffix="%" color="var(--color-success)" />
        ) : (
          <p className="text-xs text-muted-fg">No process data</p>
        )}
      </CardContent>
    </Card>
  );
}
