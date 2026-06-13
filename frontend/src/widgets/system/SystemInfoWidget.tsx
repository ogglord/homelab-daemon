import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useOverview } from "@/hooks/use-overview";
import { Info, Clock, Package } from "lucide-react";

export function SystemInfoWidget() {
  const { data: stats } = useOverview((o) => o?.Stats);

  if (!stats) {
    return (
      <Skeleton isLoading>
        <Card className="h-full"><CardContent>{"."}</CardContent></Card>
      </Skeleton>
    );
  }

  const sysRows: [string, string][] = [
    ["OS", stats.System.OS],
    ["Kernel", stats.System.KernelVersion],
    ["CPU", stats.System.CPUModel],
    ...(stats.System.Motherboard ? [["Board", stats.System.Motherboard] as [string, string]] : []),
  ];

  return (
    <Card className="h-full [--gutter:--spacing(3)]">
      <CardContent>
        {/* Header row */}
        <div className="flex items-center gap-1.5 mb-2">
          <Info className="h-3.5 w-3.5 text-primary" />
          <span className="text-[10px] tracking-[0.15em] uppercase text-primary/60 font-semibold">System Info</span>
        </div>

        {/* System info rows */}
        <div className="space-y-1">
          {sysRows.map(([key, value]) => (
            <div key={key} className="flex justify-between gap-2 text-xs">
              <span className="text-muted-fg shrink-0">{key}</span>
              <span className="font-mono text-fg truncate text-right" title={String(value)}>{value}</span>
            </div>
          ))}
        </div>

        {/* Status section — separated visually */}
        <div className="pt-1.5 mt-1.5 border-t border-border space-y-1">
          <div className="flex justify-between gap-2 text-xs">
            <span className="flex items-center gap-1 text-muted-fg shrink-0">
              <Clock className="h-3 w-3" />
              Uptime
            </span>
            <span className="font-mono text-fg text-right">{stats.UptimeStr}</span>
          </div>
          <div className="flex justify-between gap-2 text-xs">
            <span className="flex items-center gap-1 text-muted-fg shrink-0">
              <Package className="h-3 w-3" />
              Packages
            </span>
            <span className="font-mono text-fg text-right">{stats.System.Packages.toLocaleString()}</span>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
