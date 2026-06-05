import { useState, useEffect } from "react";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Meter, MeterTrack } from "@/components/ui/meter";
import { Skeleton } from "@/components/ui/skeleton";
import { useOverview } from "@/hooks/use-overview";
import type { Stats } from "@/types";

export function GpuEnginesWidget() {
  const { data: stats } = useOverview((o) => o?.Stats);

  if (!stats) return <Skeleton isLoading><Card className="h-full"><CardContent>{"."}</CardContent></Card></Skeleton>;
  if (!stats.Gpu?.Available) return null;

  return (
    <Card className="h-full">
      <CardHeader title="iGPU Engines" />
      <CardContent className="space-y-4">
        <div>
          <div className="flex justify-between text-sm mb-1">
            <span className="text-fg">Render / 3D</span>
            <span className="font-mono text-muted-fg">{stats.Gpu.RenderBusy.toFixed(1)}%</span>
          </div>
          <Meter value={stats.Gpu.RenderBusy} valueLabel={`${stats.Gpu.RenderBusy.toFixed(1)}%`} color="var(--color-primary)">
            <MeterTrack />
          </Meter>
        </div>
        <div>
          <div className="flex justify-between text-sm mb-1">
            <span className="text-fg">Video</span>
            <span className="font-mono text-muted-fg">{stats.Gpu.VideoBusy.toFixed(1)}%</span>
          </div>
          <Meter value={stats.Gpu.VideoBusy} valueLabel={`${stats.Gpu.VideoBusy.toFixed(1)}%`} color="var(--color-accent)">
            <MeterTrack />
          </Meter>
        </div>
      </CardContent>
    </Card>
  );
}
