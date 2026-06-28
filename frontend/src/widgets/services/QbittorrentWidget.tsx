import { Card, CardContent } from "@/components/ui/card";
import { Meter, MeterTrack } from "@/components/ui/meter";
import { Skeleton } from "@/components/ui/skeleton";
import { Download } from "lucide-react";
import { useOverview } from "@/hooks/use-overview";

function formatBytes(bytes: number) {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
}

export function QbittorrentWidget() {
  const { data: stats } = useOverview((o) => o?.Qbittorrent);

  if (!stats) return <Skeleton isLoading><Card className="h-full"><CardContent>{"."}</CardContent></Card></Skeleton>;
  if (!stats.enabled) return null;
  if (stats.error) {
    return (
      <Card className="h-full [--gutter:--spacing(3)]">
        <CardContent>
          <div className="flex items-center gap-1.5 mb-2">
            <Download className="h-3.5 w-3.5 text-primary" />
            <span className="text-[10px] tracking-[0.15em] uppercase text-primary/60 font-semibold">Torrents</span>
          </div>
          <p className="text-xs text-danger">{stats.error}</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="h-full [--gutter:--spacing(3)]">
      <CardContent>
        {/* Header row */}
        <div className="flex items-center gap-1.5 mb-2">
          <Download className="h-3.5 w-3.5 text-primary" />
          <span className="text-[10px] tracking-[0.15em] uppercase text-primary/60 font-semibold">Torrents</span>
          {stats.torrents?.length > 0 && (
            <span className="ml-auto font-mono text-[10px] text-muted-fg">{stats.torrents.length}</span>
          )}
        </div>

        {stats.torrents?.length === 0 ? (
          <p className="text-xs text-muted-fg">No torrents</p>
        ) : (
          <div className="space-y-1.5 max-h-64 overflow-y-auto">
            {stats.torrents?.map((t) => {
              const isDownloading = t.dlspeed > 0 || t.state.includes("downloading");
              const color = isDownloading ? "var(--color-primary)" : "var(--color-success)";
              const speed = isDownloading ? t.dlspeed : t.upspeed;
              const speedLabel = isDownloading ? "↓" : "↑";
              const progressPct = Math.min(t.progress * 100, 100);

              return (
                <div key={`${t.instance}/${t.hash}`}>
                  <div className="flex justify-between text-xs mb-0.5">
                    <span className="text-fg truncate pr-2" title={`${t.name} — ${t.instance}`}>{t.name}</span>
                    <span className="font-mono text-muted-fg shrink-0">
                      {speedLabel} {formatBytes(speed)}/s
                    </span>
                  </div>
                  <Meter value={progressPct} color={color}>
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
