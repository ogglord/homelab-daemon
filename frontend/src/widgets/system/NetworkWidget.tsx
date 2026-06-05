import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Sparkline } from "@/components/ui/sparkline";
import { useOverview } from "@/hooks/use-overview";
import { useSparkline } from "@/hooks/use-sparkline";
import { Network } from "lucide-react";

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
}

export function NetworkWidget() {
  const { data: stats } = useOverview((o) => o?.Stats);
  const recvHistory = useSparkline((o) => o.Stats?.NetRecvRate, 30);
  const sentHistory = useSparkline((o) => o.Stats?.NetSentRate, 30);

  if (!stats) return <Skeleton isLoading><Card className="h-full"><CardContent>{"."}</CardContent></Card></Skeleton>;

  return (
    <Card className="h-full [--gutter:--spacing(3)]">
      <CardContent>
        {/* Header row */}
        <div className="flex items-center gap-1.5 mb-2">
          <Network className="h-3.5 w-3.5 text-primary" />
          <span className="text-[10px] tracking-widest uppercase text-muted-fg font-medium">Network</span>
        </div>

        {/* Two columns: down / up */}
        <div className="grid grid-cols-2 gap-3">
          <div>
            <div className="flex items-center justify-between mb-1">
              <span className="text-[10px] text-muted-fg">↓ Down</span>
              <span className="text-xs font-mono font-bold text-primary">{formatBytes(stats.NetRecvRate)}/s</span>
            </div>
            <Sparkline
              data={recvHistory}
              width={160}
              height={24}
              color="var(--color-primary)"
              filled
              className="w-full"
            />
          </div>
          <div>
            <div className="flex items-center justify-between mb-1">
              <span className="text-[10px] text-muted-fg">↑ Up</span>
              <span className="text-xs font-mono font-bold text-success">{formatBytes(stats.NetSentRate)}/s</span>
            </div>
            <Sparkline
              data={sentHistory}
              width={160}
              height={24}
              color="var(--color-success)"
              filled
              className="w-full"
            />
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
