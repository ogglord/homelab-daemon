import { useState, useEffect } from "react";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Meter, MeterTrack } from "@/components/ui/meter";
import { Skeleton } from "@/components/ui/skeleton";
import { useOverview } from "@/hooks/use-overview";
import type { Stats, ProcessStat } from "@/types";

function ProcessList({ title, processes, valueKey, valueSuffix, color }: {
  title: string;
  processes: ProcessStat[];
  valueKey: "CPU" | "Memory";
  valueSuffix: string;
  color: string;
}) {
  if (!processes?.length) {
    return (
      <Card className="h-full">
        <CardHeader title={title} />
        <CardContent>
          <p className="text-sm text-muted-fg">No process data available</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="h-full flex flex-col">
      <CardHeader title={title} />
      <CardContent className="flex-1 space-y-4">
        {processes.slice(0, 5).map((p) => {
          const val = p[valueKey];
          // Cap at 100 for the progress bar visual, even if val > 100
          const pct = Math.min(val, 100);
          return (
            <div key={p.PID}>
              <div className="flex justify-between text-sm mb-1">
                <span className="text-fg truncate pr-2 font-medium">{p.Name}</span>
                <span className="font-mono text-muted-fg">{val.toFixed(1)}{valueSuffix}</span>
              </div>
              <Meter value={pct} color={color}>
                <MeterTrack />
              </Meter>
            </div>
          );
        })}
      </CardContent>
    </Card>
  );
}

export function TopCpuWidget() {
  const { data: stats } = useOverview((o) => o?.Stats);

  if (!stats) return <Skeleton isLoading><Card className="h-full"><CardContent>{"."}</CardContent></Card></Skeleton>;

  return <ProcessList title="Top CPU Processes" processes={stats.TopCPU} valueKey="CPU" valueSuffix="%" color="var(--color-primary)" />;
}
