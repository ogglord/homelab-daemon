import { useState, useEffect } from "react";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useOverview } from "@/hooks/use-overview";
import type { Stats } from "@/types";

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
}

export function NetworkWidget() {
  const { data: stats } = useOverview((o) => o?.Stats);

  if (!stats) return <Skeleton isLoading><Card className="h-full"><CardContent>{"."}</CardContent></Card></Skeleton>;

  return (
    <Card className="h-full">
      <CardHeader title="Network Throughput" />
      <CardContent>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <p className="text-xs text-muted-fg mb-1">Download</p>
            <p className="text-lg font-mono text-primary">{formatBytes(stats.NetRecvRate)}/s</p>
          </div>
          <div>
            <p className="text-xs text-muted-fg mb-1">Upload</p>
            <p className="text-lg font-mono text-accent">{formatBytes(stats.NetSentRate)}/s</p>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
