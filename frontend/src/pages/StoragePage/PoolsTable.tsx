import { ChevronRight, HardDrive } from "lucide-react";
import { Table, TableHeader, TableRow, TableColumn, TableBody, TableCell } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Meter, MeterTrack, MeterValue } from "@/components/ui/meter";
import { StatusDot } from "@/components/ui/status-dot";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import type { StorageDisk, StoragePool, StorageStatus } from "@/types";
import { fmtBytes, fmtSpeed, type DiskIORates } from "./format";

export interface PoolsTableProps {
  status: StorageStatus | null;
  expandedPools: Set<string>;
  ioRates: Map<string, DiskIORates>;
  actionLoading: boolean;
  onToggleExpand: (uuid: string) => void;
  onMount: (pool: StoragePool) => void;
  onUnmount: (pool: StoragePool) => void;
  onInitFolders: (pool: StoragePool) => void;
}

function renderDiskRole(disk: StorageDisk) {
  if (!disk.data_target && !disk.metadata_target) {
    return <span className="text-xs text-muted-fg">\u2014</span>;
  }
  const friendly = (target?: string) => {
    if (target === "foreground") return "Foreground";
    if (target === "background") return "Background";
    return target || "none";
  };
  if (disk.data_target === disk.metadata_target) {
    return <span className="text-xs">{friendly(disk.data_target)}</span>;
  }
  return (
    <span className="text-xs">
      D:{friendly(disk.data_target)} M:{friendly(disk.metadata_target)}
    </span>
  );
}

function sumPoolRates(pool: StoragePool, ioRates: Map<string, DiskIORates>) {
  let readBps = 0, writeBps = 0, iops = 0;
  pool.disks.forEach((d) => {
    const rates = ioRates.get(d.name);
    if (rates) {
      readBps += rates.readBps;
      writeBps += rates.writeBps;
      iops += rates.readIops + rates.writeIops;
    }
  });
  return { readBps, writeBps, iops };
}

export function PoolsTable(props: PoolsTableProps) {
  const {
    status, expandedPools, ioRates, actionLoading,
    onToggleExpand, onMount, onUnmount, onInitFolders,
  } = props;

  if (!status?.pools || status.pools.length === 0) {
    return (
      <div className="py-12 text-center">
        <p className="text-muted-fg italic">No bcachefs storage pools detected.</p>
      </div>
    );
  }

  return (
    <Table className="mt-4" aria-label="Storage Pools and Disks">
      <TableHeader>
        <TableColumn isRowHeader>Pool / Disk</TableColumn>
        <TableColumn>State</TableColumn>
        <TableColumn>Mountpoint</TableColumn>
        <TableColumn>Capacity</TableColumn>
        <TableColumn>Used</TableColumn>
        <TableColumn>Free</TableColumn>
        <TableColumn>Usage</TableColumn>
        <TableColumn className="w-24">Read</TableColumn>
        <TableColumn className="w-24">Write</TableColumn>
        <TableColumn className="w-16">IOPS</TableColumn>
        <TableColumn>Actions</TableColumn>
      </TableHeader>
      <TableBody>
        {status.pools.flatMap((pool) => {
          const poolRates = sumPoolRates(pool, ioRates);
          const hasUsage = !!pool.usage;
          const usedPct = pool.usage?.used_percent ?? 0;
          const mounted = pool.state === "mounted";

          const rows = [
            <TableRow id={pool.uuid} key={pool.uuid}>
              <TableCell>
                <div className="flex items-center gap-2">
                  <Button
                    intent="secondary"
                    className="p-1 min-w-0 border-none bg-transparent hover:bg-slate-200 dark:hover:bg-slate-800"
                    onPress={() => onToggleExpand(pool.uuid)}
                  >
                    <ChevronRight className={`w-4 h-4 transition-transform duration-200 ${expandedPools.has(pool.uuid) ? "rotate-90" : ""}`} />
                  </Button>
                  <span className="text-xs">{pool.name || pool.uuid}</span>
                </div>
              </TableCell>
              <TableCell>
                <div className="flex items-center gap-2 text-xs">
                  <span className="inline-flex items-center gap-1.5">
                    <StatusDot intent={mounted ? "success" : "secondary"} />
                    {mounted ? "Mounted" : "Unmounted"}
                  </span>
                  {(pool.data_replicas !== undefined || pool.metadata_replicas !== undefined) && (
                    <span>D:{pool.data_replicas ?? "?"} M:{pool.metadata_replicas ?? "?"}</span>
                  )}
                </div>
              </TableCell>
              <TableCell className="text-xs max-w-[150px] truncate">{pool.mountdir || "\u2014"}</TableCell>
              <TableCell className="text-xs">{hasUsage ? fmtBytes(pool.usage!.total_bytes) : "\u2014"}</TableCell>
              <TableCell className="text-xs">{hasUsage ? fmtBytes(pool.usage!.used_bytes) : "\u2014"}</TableCell>
              <TableCell className="text-xs">{hasUsage ? fmtBytes(pool.usage!.available_bytes) : "\u2014"}</TableCell>
              <TableCell>
                {hasUsage ? (
                  <Meter value={usedPct} color={usedPct > 90 ? "var(--color-danger)" : usedPct > 75 ? "var(--color-warning)" : "var(--color-success)"}>
                    <div className="flex items-center gap-2 min-w-[120px]">
                      <MeterValue className="text-xs tabular-nums w-9 shrink-0" />
                      <MeterTrack className="flex-1" />
                    </div>
                  </Meter>
                ) : <span className="text-xs">\u2014</span>}
              </TableCell>
              <TableCell className="text-xs tabular-nums">{mounted ? fmtSpeed(poolRates.readBps) : "\u2014"}</TableCell>
              <TableCell className="text-xs tabular-nums">{mounted ? fmtSpeed(poolRates.writeBps) : "\u2014"}</TableCell>
              <TableCell className="text-xs tabular-nums">{mounted ? `${Math.round(poolRates.iops)}` : "\u2014"}</TableCell>
              <TableCell>
                <div className="flex items-center gap-1.5">
                  {mounted ? (
                    <>
                      <Tooltip>
                        <TooltipTrigger>
                          <Button intent="secondary" size="xs" onPress={() => onInitFolders(pool)} isDisabled={actionLoading}>Init</Button>
                        </TooltipTrigger>
                        <TooltipContent>Set up the standard pool layout: media library, immich, tmp, and backup subvolumes with correct permissions.</TooltipContent>
                      </Tooltip>
                      <Tooltip>
                        <TooltipTrigger>
                          <Button intent="danger" size="xs" onPress={() => onUnmount(pool)} isDisabled={actionLoading}>Unmount</Button>
                        </TooltipTrigger>
                        <TooltipContent>Unmount the pool. Services depending on it must be stopped first.</TooltipContent>
                      </Tooltip>
                    </>
                  ) : (
                    <>
                      <Tooltip>
                        <TooltipTrigger>
                          <Button intent="primary" size="xs" onPress={() => onMount(pool)} isDisabled={actionLoading}>Mount</Button>
                        </TooltipTrigger>
                        <TooltipContent>Mount the pool at {pool.mountdir || "/pool"}.</TooltipContent>
                      </Tooltip>
                    </>
                  )}
                </div>
              </TableCell>
            </TableRow>
          ];

          if (expandedPools.has(pool.uuid)) {
            pool.disks.forEach((disk) => {
              const r = ioRates.get(disk.name) || { readBps: 0, writeBps: 0, readIops: 0, writeIops: 0 };
              const hasIO = !!disk.io;
              rows.push(
                <TableRow id={`${pool.uuid}-disk-${disk.name}`} key={`${pool.uuid}-disk-${disk.name}`}
                  className="bg-slate-50/40 dark:bg-slate-900/10 border-l-2 border-l-blue-500/50">
                  <TableCell>
                    <div className="pl-6 flex items-center gap-2">
                      <HardDrive className="w-4 h-4 text-muted-fg shrink-0" />
                      <span className="text-xs" title={disk.path}>{disk.name}</span>
                    </div>
                  </TableCell>
                  <TableCell className="text-xs">{disk.bcachefs_label || "\u2014"}</TableCell>
                  <TableCell className="text-xs max-w-[150px] truncate">{disk.friendly_name || disk.model || "\u2014"}</TableCell>
                  <TableCell className="text-xs">{fmtBytes(disk.size)}</TableCell>
                  <TableCell className="text-xs">{disk.data_target || disk.metadata_target ? renderDiskRole(disk) : "\u2014"}</TableCell>
                  <TableCell className="text-xs">{hasIO ? disk.io!.io_in_progress : "\u2014"}</TableCell>
                  <TableCell className="text-xs">\u2014</TableCell>
                  <TableCell className="text-xs tabular-nums">{hasIO ? fmtSpeed(r.readBps) : "\u2014"}</TableCell>
                  <TableCell className="text-xs tabular-nums">{hasIO ? fmtSpeed(r.writeBps) : "\u2014"}</TableCell>
                  <TableCell className="text-xs tabular-nums">{hasIO ? `${Math.round(r.readIops + r.writeIops)}` : "\u2014"}</TableCell>
                  <TableCell className="text-xs">\u2014</TableCell>
                </TableRow>
              );
            });
          }
          return rows;
        })}
      </TableBody>
    </Table>
  );
}
