import { useState, useEffect } from "react";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Meter, MeterTrack } from "@/components/ui/meter";
import { Skeleton } from "@/components/ui/skeleton";
import { useOverview } from "@/hooks/use-overview";
import type { Stats } from "@/types";

export function DisksWidget() {
  const { data: stats } = useOverview((o) => o?.Stats);

  if (!stats) return <Skeleton isLoading><Card className="h-full"><CardContent>{"."}</CardContent></Card></Skeleton>;
  const hasDisks = stats.Disks && stats.Disks.length > 0;
  if (!hasDisks) return null;

  return (
    <Card className="h-full">
      <CardHeader title="System Disks" />
      <CardContent className="space-y-4">
        {stats.Disks.map((disk) => {
          const color = disk.Percent > 90 ? "var(--color-danger)" : disk.Percent > 75 ? "var(--color-warning)" : "var(--color-success)";
          return (
            <div key={disk.Mountpoint}>
              <div className="flex justify-between text-sm mb-1">
                <span className="text-fg">{disk.Mountpoint}</span>
                <span className="font-mono text-muted-fg">{disk.UsedStr} / {disk.TotalStr}</span>
              </div>
              <Meter value={disk.Percent} valueLabel={`${disk.Percent.toFixed(1)}%`} color={color}>
                <MeterTrack />
              </Meter>
            </div>
          );
        })}
      </CardContent>
    </Card>
  );
}
