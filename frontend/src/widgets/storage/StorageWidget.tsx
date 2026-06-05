import { useState, useEffect } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { Meter, MeterTrack } from "@/components/ui/meter";
import { Skeleton } from "@/components/ui/skeleton";
import type { StorageStatus } from "@/types";
import { Database } from "lucide-react";

export function StorageWidget() {
  const [status, setStatus] = useState<StorageStatus | null>(null);

  useEffect(() => {
    const fetchStorage = () => {
      fetch("/api/storage")
        .then((r) => r.json())
        .then(setStatus)
        .catch(() => {});
    };
    fetchStorage();
    const interval = setInterval(fetchStorage, 5000);
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

  const fmtBytes = (bytes: number) => {
    if (bytes === 0) return "0 B";
    const sizes = ["B", "KB", "MB", "GB", "TB", "PB"];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${sizes[i]}`;
  };

  return (
    <Card className="h-full [--gutter:--spacing(3)]">
      <CardContent>
        {/* Header row */}
        <div className="flex items-center gap-1.5 mb-2">
          <Database className="h-3.5 w-3.5 text-primary" />
          <span className="text-[10px] tracking-widest uppercase text-muted-fg font-medium">Storage</span>
        </div>

        {pools.length === 0 ? (
          <p className="text-xs text-muted-fg">No pools found.</p>
        ) : (
          <div className="space-y-2">
            {pools.map((pool) => {
              const usedPct = pool.usage?.used_percent ?? 0;
              const color = usedPct > 90 ? "var(--color-danger)" : usedPct > 75 ? "var(--color-warning)" : "var(--color-success)";
              const total = pool.usage ? fmtBytes(pool.usage.total_bytes) : "?";
              const used = pool.usage ? fmtBytes(pool.usage.used_bytes) : "?";

              return (
                <div key={pool.uuid}>
                  <div className="flex justify-between text-xs mb-0.5">
                    <span className="text-fg font-mono truncate" title={pool.mountdir}>{pool.mountdir}</span>
                    <span className="font-mono text-muted-fg shrink-0 ml-2">{used}/{total}</span>
                  </div>
                  <Meter value={usedPct} valueLabel={`${usedPct.toFixed(1)}%`} color={color}>
                    <MeterTrack className="[--meter-height:--spacing(1)]" />
                  </Meter>
                </div>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
