import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Sparkline } from "@/components/ui/sparkline";
import { useOverview } from "@/hooks/use-overview";
import { useSparkline } from "@/hooks/use-sparkline";
import { MemoryStick } from "lucide-react";

export function MemoryWidget() {
  const { data: stats } = useOverview((o) => o?.Stats);
  const history = useSparkline((o) => o.Stats?.MemPercent, 40);

  if (!stats) return <Skeleton isLoading><Card className="h-full"><CardContent>{"."}</CardContent></Card></Skeleton>;

  return (
    <Card className="h-full [--gutter:--spacing(3)]">
      <CardContent className="relative">
        {/* Header row */}
        <div className="flex items-center justify-between mb-1">
          <div className="flex items-center gap-1.5">
            <MemoryStick className="h-3.5 w-3.5 text-success" />
            <span className="text-[10px] tracking-widest uppercase text-muted-fg font-medium">Memory</span>
          </div>
          <span className="text-[10px] font-mono text-muted-fg">{stats.MemTotalStr}</span>
        </div>

        {/* Chart area with overlaid value */}
        <div className="relative mt-1">
          <Sparkline
            data={history}
            width={400}
            height={56}
            color="var(--color-success)"
            filled
            max={100}
            className="w-full"
          />
          <div className="absolute inset-0 flex items-center justify-center">
            <span className="text-2xl font-mono font-bold text-fg drop-shadow-sm">
              {stats.MemUsedStr}
            </span>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
