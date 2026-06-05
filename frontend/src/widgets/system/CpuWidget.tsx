import { useState, useEffect } from "react";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Meter, MeterTrack } from "@/components/ui/meter";
import { Skeleton } from "@/components/ui/skeleton";
import { useOverview } from "@/hooks/use-overview";
import type { Stats } from "@/types";

export function CpuWidget() {
  const { data: stats } = useOverview((o) => o?.Stats);

  if (!stats) return <Skeleton isLoading><Card className="h-full"><CardContent>{"."}</CardContent></Card></Skeleton>;

  return (
    <Card className="h-full">
      <CardHeader title="CPU Usage" />
      <CardContent className="space-y-3">
        <div className="text-3xl font-mono font-bold text-primary">
          {stats.CPUUsage.toFixed(1)}%
        </div>
        <p className="text-sm text-muted-fg">of {stats.System.CPUCores} {stats.System.CPUCores === 1 ? "core" : "cores"}</p>
        <Meter value={stats.CPUUsage} valueLabel={`${stats.CPUUsage.toFixed(1)}%`} color="var(--color-primary)">
          <MeterTrack />
        </Meter>
      </CardContent>
    </Card>
  );
}
