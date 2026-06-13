import { useState, useEffect } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { Meter, MeterTrack } from "@/components/ui/meter";
import { Skeleton } from "@/components/ui/skeleton";
import { useOverview } from "@/hooks/use-overview";
import type { StorageStatus, DiskStat } from "@/types";
import { Database, HardDrive } from "lucide-react";

function fmtBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const sizes = ["B", "KB", "MB", "GB", "TB", "PB"];
  const i = Math.floor(Math.log(Math.abs(bytes)) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${sizes[i]}`;
}

function UsageBar({ label, used, total, percent }: { label: string; used: string; total: string; percent: number }) {
  const color = percent > 90 ? "var(--color-danger)" : percent > 75 ? "var(--color-warning)" : "var(--color-success)";
  return (
    <div>
      <div className="flex justify-between text-xs mb-0.5">
        <span className="text-fg font-mono truncate">{label}</span>
        <span className="font-mono text-muted-fg shrink-0 ml-2">{used}/{total}</span>
      </div>
      <Meter value={percent} valueLabel={`${percent.toFixed(1)}%`} color={color}>
        <MeterTrack className="[--meter-height:--spacing(1)]" />
      </Meter>
    </div>
  );
}

export function StorageWidget() {
  const [poolStatus, setPoolStatus] = useState<StorageStatus | null>(null);
  const { data: stats } = useOverview((o) => o?.Stats);

  useEffect(() => {
    const fetchStorage = () => {
      fetch("/api/storage")
        .then((r) => r.json())
        .then(setPoolStatus)
        .catch(() => {});
    };
    fetchStorage();
    const interval = setInterval(fetchStorage, 5000);
    return () => clearInterval(interval);
  }, []);

  if (!poolStatus && !stats) {
    return (
      <Skeleton isLoading>
        <Card className="h-full"><CardContent>{"."}</CardContent></Card>
      </Skeleton>
    );
  }

  const pools = poolStatus?.pools || [];
  const disks = stats?.Disks || [];

  return (
    <Card className="h-full [--gutter:--spacing(3)]">
      <CardContent>
        <div className="flex items-center gap-1.5 mb-2">
          <Database className="h-3.5 w-3.5 text-primary" />
          <span className="text-[10px] tracking-[0.15em] uppercase text-primary/60 font-semibold">Storage</span>
        </div>

        <div className="space-y-2">
          {/* System disks */}
          {disks.map((d: DiskStat) => (
            <UsageBar key={d.Mountpoint}
              label={d.Mountpoint}
              used={d.UsedStr || fmtBytes(d.Used)}
              total={d.TotalStr || fmtBytes(d.Total)}
              percent={d.Percent}
            />
          ))}

          {/* bcachefs pools */}
          {pools.length > 0 && disks.length > 0 && (
            <hr className="border-border my-1.5" />
          )}

          {pools.map((pool) => {
            const usedPct = pool.usage?.used_percent ?? 0;
            const total = pool.usage ? fmtBytes(pool.usage.total_bytes) : "?";
            const used = pool.usage ? fmtBytes(pool.usage.used_bytes) : "?";
            return (
              <UsageBar key={pool.uuid}
                label={pool.mountdir || pool.name || pool.uuid}
                used={used}
                total={total}
                percent={usedPct}
              />
            );
          })}

          {pools.length === 0 && disks.length === 0 && (
            <p className="text-xs text-muted-fg">No storage detected.</p>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
