import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useOverview } from "@/hooks/use-overview";
import { Clock, Package } from "lucide-react";

export function UptimeWidget() {
  const { data: stats } = useOverview((o) => o?.Stats);

  if (!stats) return <Skeleton isLoading><Card className="h-full"><CardContent>{"."}</CardContent></Card></Skeleton>;

  return (
    <Card className="h-full [--gutter:--spacing(3)]">
      <CardContent>
        {/* Header row */}
        <div className="flex items-center gap-1.5 mb-2">
          <Clock className="h-3.5 w-3.5 text-primary" />
          <span className="text-[10px] tracking-widest uppercase text-muted-fg font-medium">Uptime</span>
        </div>

        {/* Two-stat compact layout */}
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-xs text-muted-fg">Uptime</span>
            <span className="text-sm font-mono font-bold text-fg">{stats.UptimeStr}</span>
          </div>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-1">
              <Package className="h-3 w-3 text-muted-fg" />
              <span className="text-xs text-muted-fg">Packages</span>
            </div>
            <span className="text-sm font-mono font-bold text-fg">{stats.System.Packages.toLocaleString()}</span>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
