import { Card, CardContent } from "@/components/ui/card";
import { Meter, MeterTrack } from "@/components/ui/meter";
import { Skeleton } from "@/components/ui/skeleton";
import { useOverview } from "@/hooks/use-overview";
import { HardDrive } from "lucide-react";

export function DisksWidget() {
  const { data: stats } = useOverview((o) => o?.Stats);

  if (!stats) return <Skeleton isLoading><Card className="h-full"><CardContent>{"."}</CardContent></Card></Skeleton>;
  const hasDisks = stats.Disks && stats.Disks.length > 0;
  if (!hasDisks) return null;

  return (
    <Card className="h-full [--gutter:--spacing(3)]">
      <CardContent>
        {/* Header row */}
        <div className="flex items-center gap-1.5 mb-2">
          <HardDrive className="h-3.5 w-3.5 text-primary" />
          <span className="text-[10px] tracking-widest uppercase text-muted-fg font-medium">Disks</span>
        </div>

        {/* Disk rows */}
        <div className="space-y-2">
          {stats.Disks.map((disk) => {
            const color = disk.Percent > 90 ? "var(--color-danger)" : disk.Percent > 75 ? "var(--color-warning)" : "var(--color-success)";
            return (
              <div key={disk.Mountpoint}>
                <div className="flex justify-between text-xs mb-0.5">
                  <span className="text-fg font-mono">{disk.Mountpoint}</span>
                  <span className="font-mono text-muted-fg">{disk.UsedStr}/{disk.TotalStr}</span>
                </div>
                <Meter value={disk.Percent} valueLabel={`${disk.Percent.toFixed(1)}%`} color={color}>
                  <MeterTrack className="[--meter-height:--spacing(1)]" />
                </Meter>
              </div>
            );
          })}
        </div>
      </CardContent>
    </Card>
  );
}
