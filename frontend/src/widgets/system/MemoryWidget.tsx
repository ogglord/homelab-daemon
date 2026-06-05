import { useState, useEffect } from "react";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Meter, MeterTrack } from "@/components/ui/meter";
import { Skeleton } from "@/components/ui/skeleton";
import { useOverview } from "@/hooks/use-overview";
import type { Stats } from "@/types";

export function MemoryWidget() {
  const { data: stats } = useOverview((o) => o?.Stats);

  if (!stats) return <Skeleton isLoading><Card className="h-full"><CardContent>{"."}</CardContent></Card></Skeleton>;

  return (
    <Card className="h-full">
      <CardHeader title="Memory" />
      <CardContent className="space-y-3">
        <div className="text-3xl font-mono font-bold text-success">
          {stats.MemUsedStr}
        </div>
        <p className="text-sm text-muted-fg">of {stats.MemTotalStr}</p>
        <Meter value={stats.MemPercent} valueLabel={`${stats.MemPercent.toFixed(1)}%`} color="var(--color-success)">
          <MeterTrack />
        </Meter>
      </CardContent>
    </Card>
  );
}
