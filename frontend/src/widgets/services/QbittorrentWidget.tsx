import { useState } from "react";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Meter, MeterTrack } from "@/components/ui/meter";
import { Skeleton } from "@/components/ui/skeleton";
// qBittorrent removed in Phase 1 — widget is disabled


interface QbitTorrent {
  hash: string;
  name: string;
  size: number;
  progress: number;
  dlspeed: number;
  upspeed: number;
  eta: number;
  state: string;
}

interface QbitStats {
  torrents: QbitTorrent[];
  enabled: boolean;
  error?: string;
}

function formatBytes(bytes: number) {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

export function QbittorrentWidget() {
  const [stats, setStats] = useState<QbitStats | null>(null);


  if (!stats) return <Skeleton isLoading><Card className="h-full"><CardContent>{"."}</CardContent></Card></Skeleton>;
  if (!stats.enabled) return null;
  if (stats.error) {
     return (
      <Card className="h-full">
        <CardHeader title="Active Torrents" />
        <CardContent>
          <p className="text-sm text-destructive">{stats.error}</p>
        </CardContent>
      </Card>
     );
  }

  return (
    <Card className="h-full flex flex-col">
      <CardHeader title="Active Torrents" />
      <CardContent className="flex-1 space-y-4">
        {stats.torrents?.length === 0 ? (
          <p className="text-sm text-muted-fg">No active transfers</p>
        ) : (
          stats.torrents?.map((t) => {
            const isDownloading = t.dlspeed > 0 || t.state.includes("downloading");
            const color = isDownloading ? "var(--color-primary)" : "var(--color-success)";
            const speed = isDownloading ? t.dlspeed : t.upspeed;
            const speedLabel = isDownloading ? '↓' : '↑';
            const progressPct = Math.min(t.progress * 100, 100);
            
            return (
              <div key={t.hash}>
                <div className="flex justify-between text-sm mb-1">
                  <span className="text-fg truncate pr-2 font-medium" title={t.name}>{t.name}</span>
                  <span className="font-mono text-muted-fg whitespace-nowrap">
                    {speedLabel} {formatBytes(speed)}/s
                  </span>
                </div>
                <Meter value={progressPct} color={color}>
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
