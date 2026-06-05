import { useState, useEffect } from "react";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Meter, MeterTrack } from "@/components/ui/meter";
import { Skeleton } from "@/components/ui/skeleton";
import type { StorageStatus } from "@/types";

export function StorageWidget() {
  const [status, setStatus] = useState<StorageStatus | null>(null);

  useEffect(() => {
    fetch("/api/storage")
      .then((r) => r.json())
      .then(setStatus)
      .catch(() => {});
      
    const interval = setInterval(() => {
      fetch("/api/storage")
        .then((r) => r.json())
        .then(setStatus)
        .catch(() => {});
    }, 5000);
    return () => clearInterval(interval);
  }, []);

  if (!status) {
    return (
      <Skeleton isLoading>
        <Card className="h-full"><CardContent>{"."}</CardContent></Card>
      </Skeleton>
    );
  }

  const pools = status.pools || [];
  
  return (
    <Card className="h-full">
      <CardHeader title="Storage Pools" />
      <CardContent className="space-y-4">
        {pools.length === 0 ? (
          <p className="text-sm text-muted-fg">No storage pools found.</p>
        ) : (
          pools.map((pool) => {
            const usedPct = pool.usage?.used_percent ?? 0;
            const color = usedPct > 90 ? "var(--color-danger)" : usedPct > 75 ? "var(--color-warning)" : "var(--color-success)";
            
            const fmtBytes = (bytes: number) => {
              if (bytes === 0) return "0 B";
              const sizes = ["B", "KB", "MB", "GB", "TB", "PB"];
              const i = Math.floor(Math.log(bytes) / Math.log(1024));
              return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${sizes[i]}`;
            };
            
            const total = pool.usage ? fmtBytes(pool.usage.total_bytes) : "Unknown";
            const used = pool.usage ? fmtBytes(pool.usage.used_bytes) : "Unknown";

            // Build a metadata line: "N disks · replicas=N · state"
            const metaParts: string[] = [];
            if (pool.disks?.length) {
              metaParts.push(`${pool.disks.length} disk${pool.disks.length > 1 ? 's' : ''}`);
            }
            if (pool.data_replicas !== undefined) {
              metaParts.push(`replicas=${pool.data_replicas}`);
            }
            metaParts.push(pool.state);
            const metaLine = metaParts.join(' · ');

            return (
              <div key={pool.uuid}>
                <div className="flex justify-between text-sm mb-1">
                  <div className="min-w-0">
                    <span className="text-fg truncate block" title={pool.mountdir}>{pool.mountdir}</span>
                    <span className="text-xs text-muted-fg">{metaLine}</span>
                  </div>
                  <span className="font-mono text-muted-fg whitespace-nowrap ml-2">{used} / {total}</span>
                </div>
                <Meter value={usedPct} valueLabel={`${usedPct.toFixed(1)}%`} color={color}>
                  <MeterTrack />
                </Meter>
              </div>
            );
          })
        )}
      </CardContent>
    </Card>
  );
}
