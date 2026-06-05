import { useState, useEffect, useRef } from "react";
import { toast } from "sonner";
import { RefreshCw } from "lucide-react";
import { CardHeader, CardTitle, CardDescription, CardAction } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import type { StorageStatus, StoragePool, DiskIOStats } from "@/types";
import { PoolsTable } from "./StoragePage/PoolsTable";
import { UnassignedDisks } from "./StoragePage/UnassignedDisks";
import { SubvolumesPanel } from "./StoragePage/SubvolumesPanel";
import type { DiskIORates } from "./StoragePage/format";

export function StoragePage() {
  const [status, setStatus] = useState<StorageStatus | null>(null);
  const [queryLoading, setQueryLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState(false);
  const [expandedPools, setExpandedPools] = useState<Set<string>>(new Set());
  const [ioRates, setIoRates] = useState<Map<string, DiskIORates>>(new Map());
  const [subvolumeSizes, setSubvolumeSizes] = useState<Map<string, number>>(new Map());
  const [calculatingUsage, setCalculatingUsage] = useState(false);
  const prevIO = useRef<Map<string, { stats: DiskIOStats; time: number }>>(new Map());

  const fetchStorage = async () => {
    try {
      const res = await fetch("/api/storage");
      if (!res.ok) throw new Error(await res.text());
      const data = (await res.json()) as StorageStatus;
      setStatus(data);
      setQueryLoading(false);
    } catch (err) {
      toast.error(`Failed to fetch storage status: ${err}`);
      setQueryLoading(false);
    }
  };

  useEffect(() => {
    fetchStorage();
    const interval = setInterval(fetchStorage, 3000);
    return () => clearInterval(interval);
  }, []);

  // Compute IO rates from successive polls.
  useEffect(() => {
    if (!status) return;
    const now = Date.now();
    for (const pool of status.pools || []) {
      for (const disk of pool.disks || []) {
        if (!disk.io) continue;
        const prev = prevIO.current.get(disk.name);
        if (prev && prev.stats) {
          const dt = (now - prev.time) / 1000;
          if (dt > 0) {
            const dr = ((disk.io.read_sectors - prev.stats.read_sectors) || 0) * 512;
            const dw = ((disk.io.write_sectors - prev.stats.write_sectors) || 0) * 512;
            setIoRates((prevMap) => {
              const next = new Map(prevMap);
              next.set(disk.name, {
                readBps: dr / dt,
                writeBps: dw / dt,
                readIops: dr > 0 ? (disk.io!.read_ios - (prev.stats.read_ios || 0)) / dt : 0,
                writeIops: dw > 0 ? (disk.io!.write_ios - (prev.stats.write_ios || 0)) / dt : 0,
              });
              return next;
            });
          }
        }
        prevIO.current.set(disk.name, { stats: disk.io, time: now });
      }
    }
  }, [status]);

  // Actions
  const execAction = async (url: string, body: unknown, successMsg: string) => {
    setActionLoading(true);
    try {
      const res = await fetch(url, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      const data = await res.json();
      if (data.success) {
        toast.success(successMsg);
        fetchStorage();
      } else {
        toast.error(data.error || "Action failed");
      }
    } catch {
      toast.error("Action failed");
    } finally {
      setActionLoading(false);
    }
  };

  const onMount = (pool: StoragePool) => {
    const devices = pool.disks?.map((d) => d.path) ?? [];
    execAction("/api/storage/mount", { devices, mountpoint: pool.mountdir || "/pool" }, `Mounted ${pool.name}`);
  };
  const onUnmount = (pool: StoragePool) => {
    execAction("/api/storage/unmount", { mountpoint: pool.mountdir }, `Unmounted ${pool.name}`);
  };
  const onInitFolders = (pool: StoragePool) => {
    execAction("/api/storage/init-folders", { mountpoint: pool.mountdir }, `Initialized folders on ${pool.name}`);
  };
  const onCreateSubvolume = () => {
    execAction("/api/storage/subvolume", { path: "/pool/tmp" }, "Created subvolume /pool/tmp");
  };
  const onDeleteSubvolume = (path: string) => {
    setActionLoading(true);
    fetch(`/api/storage/subvolume?path=${encodeURIComponent(path)}`, { method: "DELETE" })
      .then((r) => {
        if (!r.ok) throw new Error("Delete failed");
        toast.success(`Deleted subvolume ${path}`);
        fetchStorage();
      })
      .catch((e) => toast.error(e.message))
      .finally(() => setActionLoading(false));
  };

  const onCalculateUsage = async () => {
    if (!status?.subvolumes?.length) return;
    setCalculatingUsage(true);
    try {
      const paths = status.subvolumes.map((sv) => sv.path);
      const res = await fetch("/api/storage/subvolume-usage", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ paths }),
      });
      const data = await res.json();
      if (data.success) {
        setSubvolumeSizes(new Map(Object.entries(data.usage || {})));
        toast.success("Usage calculated");
      } else {
        toast.error(data.error || "Failed to calculate usage");
      }
    } catch {
      toast.error("Failed to calculate usage");
    } finally {
      setCalculatingUsage(false);
    }
  };

  const unassigned = status?.unassigned ?? [];
  const isInitialLoading = queryLoading && !status;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-fg">Storage</h1>
          <p className="text-sm text-muted-fg">
            {isInitialLoading
              ? "Loading storage status..."
              : `${(status?.pools || []).length} pool${(status?.pools || []).length !== 1 ? "s" : ""}`}
          </p>
        </div>
        <Button intent="outline" onPress={fetchStorage} isDisabled={queryLoading}>
          <RefreshCw className={`w-4 h-4 mr-2 ${queryLoading ? "animate-spin" : ""}`} />
          Refresh
        </Button>
      </div>

      {/* Pools Table */}
      <div className="rounded-lg border p-4">
        <CardHeader>
          <CardTitle>Storage Pools</CardTitle>
          <CardDescription>
            {status?.pools?.length
              ? `${status.pools.length} pool${status.pools.length !== 1 ? "s" : ""}`
              : "No pools detected"}
          </CardDescription>
        </CardHeader>
        <PoolsTable
          status={status}
          expandedPools={expandedPools}
          ioRates={ioRates}
          actionLoading={actionLoading}
          onToggleExpand={(uuid) => {
            setExpandedPools((prev) => {
              const next = new Set(prev);
              next.has(uuid) ? next.delete(uuid) : next.add(uuid);
              return next;
            });
          }}
          onMount={onMount}
          onUnmount={onUnmount}
          onInitFolders={onInitFolders}
        />
      </div>

      {/* Subvolumes + Unassigned Disks side-by-side */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <SubvolumesPanel
          status={status}
          subvolumeSizes={subvolumeSizes}
          calculatingUsage={calculatingUsage}
          actionLoading={actionLoading}
          onCalculateUsage={onCalculateUsage}
          onCreate={onCreateSubvolume}
          onDelete={onDeleteSubvolume}
        />
        <UnassignedDisks disks={unassigned} />
      </div>
    </div>
  );
}

export default StoragePage;
