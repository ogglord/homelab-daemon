import { useState, useEffect, useCallback, useMemo } from "react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { ExternalLink } from "lucide-react";
import { PageShell, useTable, useRowContextMenu } from "@/components/data-table";
import { Menu, MenuContent, MenuTrigger } from "@/components/ui/menu";
import {
  Table, TableBody, TableCell, TableColumn, TableHeader, TableRow,
} from "@/components/ui/table";
import { Switch } from "@/components/ui/switch";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useOverview } from "@/hooks/use-overview";
import type { ServiceInfo, PortMapping } from "@/types";
import { StatusDot } from "@/components/ui/status-dot";
import { StateBadge, fmtBackoff } from "./ServicesPage/StateBadge";
import { PortChips } from "./ServicesPage/PortChips";
import { PullDialog } from "./ServicesPage/PullDialog";
import { ServiceConfigSheet, type ServiceConfigDraft } from "./ServicesPage/ServiceConfigSheet";
import { ConfirmDialogs } from "./ServicesPage/ConfirmDialogs";
import { BulkControlBar } from "./ServicesPage/BulkControlBar";
import { ServiceCardsGrid } from "./ServicesPage/ServiceCardsGrid";
import {
  renderMenuItems as renderMenuItemsHelper,
  renderContextMenuItems as renderContextMenuItemsHelper,
  type ServiceActionsCallbacks,
  type ServiceActionItem,
} from "./ServicesPage/ServiceActionsMenu";

const FALLBACK_ICON = "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='currentColor' stroke-width='2'%3E%3Crect x='2' y='2' width='20' height='8' rx='2'/%3E%3Crect x='2' y='14' width='20' height='8' rx='2'/%3E%3C/svg%3E";

// Row-level view model — flat fields for sorting/filtering.
interface ServiceRow extends Record<string, unknown> {
  id: string;
  name: string;
  type: string;
  state: string;
  subState: string;
  description: string;
  unit_name: string;
  update_available: boolean;
  current_version: string;
  remote_version: string;
  image: string;
  daemon_enabled: boolean;
  restart_policy: string;
  boot_order: number;
  boot_delay: number;
  restart_delay: number;
  depends_on: string[];
  user_stopped: boolean;
  failure_count: number;
  backoff_seconds: number;
  blocked_reason: string;
  requires_mount: string[];
  icon_url: string;
  homepage_url: string;
  port_mappings: PortMapping[];
  started_at: string;
}

function toRow(svc: ServiceInfo): ServiceRow {
  return {
    id: svc.unit_name,
    name: svc.name,
    type: svc.type,
    state: svc.active_state,
    subState: svc.sub_state,
    description: svc.description || "—",
    unit_name: svc.unit_name,
    update_available: svc.update_available,
    current_version: svc.current_version || "",
    remote_version: svc.remote_version || "",
    image: svc.image || "",
    daemon_enabled: svc.daemon_enabled,
    restart_policy: svc.restart_policy,
    boot_order: svc.boot_order,
    boot_delay: svc.boot_delay,
    restart_delay: svc.restart_delay,
    depends_on: svc.depends_on,
    user_stopped: svc.user_stopped,
    failure_count: svc.failure_count,
    backoff_seconds: svc.backoff_seconds,
    blocked_reason: svc.blocked_reason || "",
    requires_mount: svc.requires_mount || [],
    icon_url: svc.icon_url || "",
    homepage_url: svc.homepage_url || "",
    port_mappings: svc.port_mappings || [],
    started_at: svc.started_at || "",
  };
}

function fmtUptime(startedAt: string): string {
  if (!startedAt) return "—";
  const diff = Math.floor((Date.now() - new Date(startedAt).getTime()) / 1000);
  if (diff < 60) return "< 1m";
  if (diff < 3600) return `${Math.floor(diff / 60)}m`;
  if (diff < 86400) {
    const h = Math.floor(diff / 3600);
    const m = Math.floor((diff % 3600) / 60);
    return m > 0 ? `${h}h ${m}m` : `${h}h`;
  }
  const d = Math.floor(diff / 86400);
  const h = Math.floor((diff % 86400) / 3600);
  return h > 0 ? `${d}d ${h}h` : `${d}d`;
}

type ViewMode = "table" | "cards";

function loadViewMode(): ViewMode {
  try {
    const v = localStorage.getItem("homelab-services-view");
    if (v === "cards" || v === "table") return v;
  } catch {}
  return "table";
}

export default function ServicesPage() {
  const navigate = useNavigate();
  const [services, setServices] = useState<ServiceInfo[] | null>(null);
  const [pull, setPull] = useState({ open: false, unit: "", name: "", image: "", autoRestart: false });
  const [loadingAction, setLoadingAction] = useState<string | null>(null);
  const [confirmAction, setConfirmAction] = useState<{ unit: string; action: string; label: string } | null>(null);
  const [confirmBulkStop, setConfirmBulkStop] = useState(false);
  const [bulkLoading, setBulkLoading] = useState<"stop" | "start" | null>(null);
  const [configDrawer, setConfigDrawer] = useState<ServiceConfigDraft | null>(null);
  const [viewMode, setViewMode] = useState<ViewMode>(loadViewMode);

  function toggleViewMode() {
    setViewMode((v) => {
      const next = v === "table" ? "cards" : "table";
      try { localStorage.setItem("homelab-services-view", next); } catch {}
      return next;
    });
  }

  // Sort & filter (from shared hook)
  const rows = useMemo(() => services?.map(toRow) ?? null, [services]);
  const {
    filterText, setFilterText,
    sortDescriptor, setSortDescriptor,
    sorted,
    totalCount,
  } = useTable<ServiceRow>({
    data: rows,
    defaultSort: { column: "name", direction: "ascending" },
    filterField: "name",
  });

  // Poll for services via the overview endpoint.
  const overview = useOverview((o) => o?.Services);

  useEffect(() => {
    if (overview.data) setServices(overview.data);
  }, [overview.data]);

  // Service failure toast from custom events (fired by useOverview)
  useEffect(() => {
    const handler = (e: CustomEvent) => {
      const info = e.detail as { name: string; failure_count: number; backoff_seconds: number };
      const desc = info.backoff_seconds > 0
        ? `Backing off — retry in ${fmtBackoff(info.backoff_seconds)}`
        : "Retrying shortly";
      toast.error(`${info.name} failed to start (attempt ${info.failure_count})`, {
        description: desc,
        duration: 8000,
      });
    };
    window.addEventListener("homelab:service-failure", handler as EventListener);
    return () => window.removeEventListener("homelab:service-failure", handler as EventListener);
  }, []);

  // Clear services on error
  useEffect(() => {
    if (overview.error) setServices(null);
  }, [overview.error]);

  const refetchServices = useCallback(() => {
    fetch("/api/v1/overview")
      .then((r) => r.json())
      .then((data) => setServices(data.Services as ServiceInfo[]))
      .catch(() => {});
  }, []);

  async function doAction(unit: string, action: string) {
    setConfirmAction(null);
    setLoadingAction(`${unit}:${action}`);
    try {
      const resp = await fetch(`/api/${action}/${unit}`, { method: "POST" });
      if (!resp.ok) {
        const body = await resp.text().catch(() => "");
        toast.error(`${action} failed: HTTP ${resp.status}${body ? ` — ${body.slice(0, 120)}` : ""}`);
        return;
      }
      const data = await resp.json();
      if (data.success) {
        toast.success(data.message ?? `${action} succeeded`);
        setTimeout(() => { refetchServices(); }, 1000);
      } else {
        toast.error(data.error ?? "Action failed");
      }
    } catch {
      toast.error(`${action} failed (network error)`);
    } finally {
      setLoadingAction(null);
    }
  }

  async function toggleAutoStart(unit: string, enabled: boolean) {
    try {
      const resp = await fetch(`/api/config/${unit}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled }),
      });
      const data = await resp.json();
      if (data.success) {
        toast.success(`AutoStart ${enabled ? "enabled" : "disabled"} for ${unit}`);
        setServices((prev) =>
          prev
            ? prev.map((s) => (s.unit_name === unit ? { ...s, daemon_enabled: enabled } : s))
            : null,
        );
      } else {
        toast.error(data.error ?? "Failed to toggle AutoStart");
      }
    } catch {
      toast.error("Failed to toggle AutoStart");
    }
  }

  async function stopAndDisable(unit: string, name: string) {
    setLoadingAction(`${unit}:stop-disable`);
    try {
      await fetch(`/api/stop/${unit}`, { method: "POST" });
      const resp = await fetch(`/api/config/${unit}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled: false }),
      });
      const data = await resp.json();
      if (data.success) {
        toast.success(`${name} stopped and disabled`);
        setServices((prev) =>
          prev
            ? prev.map((s) =>
                s.unit_name === unit
                  ? { ...s, daemon_enabled: false, active_state: "inactive", sub_state: "dead", user_stopped: true }
                  : s,
              )
            : null,
        );
      } else {
        toast.error(data.error ?? "Failed to disable");
      }
    } catch {
      toast.error("Failed to stop and disable");
    } finally {
      setLoadingAction(null);
    }
  }

  async function saveConfig() {
    if (!configDrawer) return;
    const { unit, boot_order, boot_delay, restart_delay, depends_on } = configDrawer;
    const depsArray = depends_on.split(",").map((d) => d.trim()).filter((d) => d.length > 0);

    try {
      const resp = await fetch(`/api/config/${unit}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          order: Number(boot_order),
          boot_delay: Number(boot_delay),
          restart_delay: Number(restart_delay),
          depends_on: depsArray,
        }),
      });
      const data = await resp.json();
      if (data.success) {
        toast.success(`Configuration saved for ${configDrawer.name}`);
        setConfigDrawer(null);
        refetchServices();
      } else {
        toast.error(data.error ?? "Failed to save configuration");
      }
    } catch {
      toast.error("Failed to save configuration");
    }
  }

  async function doBulkAction(action: "stop" | "start") {
    setConfirmBulkStop(false);
    setBulkLoading(action);
    try {
      const resp = await fetch(`/api/${action}-all`, { method: "POST" });
      if (!resp.ok) {
        toast.error(`Bulk ${action} failed: HTTP ${resp.status}`);
        return;
      }
      const data = await resp.json();
      if (data.failed === 0) {
        toast.success(`All services ${action === "stop" ? "stopped" : "started"} (${data.total})`);
      } else {
        toast.warning(`${data.failed}/${data.total} services failed to ${action}`);
      }
      refetchServices();
    } catch {
      toast.error(`Bulk ${action} failed`);
    } finally {
      setBulkLoading(null);
    }
  }

  const triggerUpdate = useCallback((unit: string) => {
    const svc = services?.find((s) => s.unit_name === unit);
    if (!svc) return;
    const name = unit.replace(/^podman-/, "").replace(/\.service$/, "");
    setPull({ open: true, unit, name, image: svc.image ?? "", autoRestart: true });
  }, [services]);

  const buildMenuCallbacks = useCallback(
    (afterAction?: () => void): ServiceActionsCallbacks => ({
      onStopConfirm: (unit, name) => setConfirmAction({ unit, action: "stop", label: name }),
      onStopAndDisable: stopAndDisable,
      onRestart: (unit) => doAction(unit, "restart"),
      onStart: (unit) => doAction(unit, "start"),
      onEnableAutoStart: (unit) => toggleAutoStart(unit, true),
      onShowLogs: (unit) => {
        const name = unit.replace(/^podman-/, "").replace(/\.service$/, "");
        navigate(`/logs?service=${encodeURIComponent(name)}`);
      },
      onConfigure: (item) => setConfigDrawer({
        open: true,
        unit: item.unit_name,
        name: item.name,
        boot_order: item.boot_order,
        boot_delay: item.boot_delay,
        restart_delay: item.restart_delay,
        depends_on: (item.depends_on || []).join(", "),
      }),
      onPull: (unit, image) => {
        const name = unit.replace(/^podman-/, "").replace(/\.service$/, "");
        setPull({ open: true, unit, name, image, autoRestart: false });
      },
      afterAction,
    }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [navigate],
  );

  const renderContextMenuItems = (item: ServiceActionItem) =>
    renderContextMenuItemsHelper(item, buildMenuCallbacks());

  // Inner component rendered inside PageShell so useRowContextMenu
  // has access to the TableContext provider.
  function ServicesTableContent({ sorted: items, sortDescriptor: sd, onSortChange, triggerUpdate: triggerUpdateFn, toggleAutoStart: toggleAutoStartFn }: {
    sorted: ServiceRow[];
    sortDescriptor: any;
    onSortChange: (d: any) => void;
    triggerUpdate: (unit: string) => void;
    toggleAutoStart: (unit: string, enabled: boolean) => void;
  }) {
    const onRowCtx = useRowContextMenu<ServiceRow>();
    const menuCallbacks = buildMenuCallbacks();

    return (
      <Table
        aria-label="Services"
        sortDescriptor={sd}
        onSortChange={(d: any) => onSortChange({ column: String(d.column), direction: d.direction as "ascending" | "descending" })}
      >
        <TableHeader columns={[
          { id: "icon",      name: "",           className: "w-11" },
          { id: "name",      name: "Name",       isRowHeader: true, className: "w-40 min-w-[8rem]" },
          { id: "status",    name: "Status",     className: "w-44 min-w-[11rem]" },
          { id: "uptime",    name: "Uptime",     className: "w-24 hidden sm:table-cell" },
          { id: "webports",  name: "Web / Ports", className: "hidden md:table-cell" },
          { id: "autostart", name: "AutoStart",  className: "w-24" },
        ] as Array<{ id: string; name: string; isRowHeader?: boolean; className?: string }>}>
          {(column) => (
            <TableColumn
              isRowHeader={column.id === "name"}
              allowsSorting={column.id === "name" || column.id === "status" || column.id === "uptime"}
              className={column.className ?? ""}
            >
              {column.name}
            </TableColumn>
          )}
        </TableHeader>
        <TableBody items={items}>
          {(item: ServiceRow) => (
            <TableRow onContextMenu={onRowCtx(item)}>
              {/* Icon — left-click opens action menu */}
              <TableCell className="py-1.5">
                <Menu>
                  <MenuTrigger
                    aria-label={`Actions for ${item.name}`}
                    className="flex items-center justify-center rounded-md p-0.5 outline-hidden hover:ring-1 hover:ring-border focus-visible:ring-2 focus-visible:ring-primary transition-all"
                  >
                    <img
                      src={item.icon_url || FALLBACK_ICON}
                      alt={item.name}
                      className={`size-8 object-contain rounded ${item.state !== "active" ? "grayscale opacity-50" : ""}`}
                      loading="lazy"
                      data-fallback={!item.icon_url ? "true" : undefined}
                      onError={(e) => {
                        (e.target as HTMLImageElement).src = FALLBACK_ICON;
                        (e.target as HTMLImageElement).dataset.fallback = "true";
                      }}
                    />
                  </MenuTrigger>
                  <MenuContent placement="bottom start" className="min-w-40">
                    {renderMenuItemsHelper(item, menuCallbacks)}
                  </MenuContent>
                </Menu>
              </TableCell>

              {/* Name + update dot */}
              <TableCell className="text-sm">
                <span className="inline-flex items-center gap-1.5">
                  {item.name}
                  {item.update_available && (
                    <Tooltip>
                      <TooltipTrigger>
                        <StatusDot
                          intent="warning"
                          className="cursor-pointer"
                          onClick={() => triggerUpdateFn(item.unit_name)}
                        />
                      </TooltipTrigger>
                      <TooltipContent>
                        Update available — click to pull {item.image || item.name} and restart.
                        {item.remote_version && ` Remote: ${item.remote_version}.`}
                      </TooltipContent>
                    </Tooltip>
                  )}
                </span>
              </TableCell>

              {/* Status */}
              <TableCell>
                <StateBadge
                  state={item.state}
                  subState={item.subState}
                  userStopped={item.user_stopped}
                  failureCount={item.failure_count}
                  backoffSeconds={item.backoff_seconds}
                  blockedReason={item.blocked_reason}
                />
              </TableCell>

              {/* Uptime */}
              <TableCell className="text-sm text-muted-fg tabular-nums hidden sm:table-cell">
                {fmtUptime(item.started_at)}
              </TableCell>

              {/* Web / Ports */}
              <TableCell className="hidden md:table-cell">
                <span className="inline-flex flex-wrap items-center gap-1.5">
                  {item.homepage_url && (
                    <a
                      href={item.homepage_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
                      onClick={(e) => e.stopPropagation()}
                    >
                      <ExternalLink className="size-3 shrink-0" />
                      <span className="max-w-[12rem] truncate">
                        {(() => { try { return new URL(item.homepage_url).hostname; } catch { return item.homepage_url; } })()}
                      </span>
                    </a>
                  )}
                  <PortChips ports={item.port_mappings} />
                </span>
              </TableCell>

              {/* AutoStart */}
              <TableCell>
                <Switch
                  isSelected={item.daemon_enabled}
                  onChange={(selected: boolean) => toggleAutoStartFn(item.unit_name, selected)}
                  aria-label="AutoStart toggle"
                />
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    );
  }

  const activeCount = services?.filter((s) => s.active_state === "active").length ?? 0;

  const bulkBar = (
    <BulkControlBar
      activeCount={activeCount}
      totalCount={totalCount}
      bulkLoading={bulkLoading}
      viewMode={viewMode}
      onStartAll={() => doBulkAction("start")}
      onRequestStopAll={() => setConfirmBulkStop(true)}
      onToggleView={toggleViewMode}
    />
  );

  return (
    <PageShell<ServiceRow>
      data={rows}
      title="Services"
      description={(n, f) => `${n} service${n !== 1 ? "s" : ""}${f ? ` matching "${f}"` : ""}`}
      searchable
      filter={filterText}
      onFilterChange={setFilterText}
      emptyMessage="No services found"
      emptyDescription="Services will appear here when configured"
      contextMenu={renderContextMenuItems}
      beforeTable={bulkBar}
    >
      {viewMode === "cards" ? (
        <ServiceCardsGrid
          items={sorted}
          buildMenuCallbacks={buildMenuCallbacks}
          toggleAutoStart={toggleAutoStart}
          triggerUpdate={triggerUpdate}
          renderContextMenuItems={renderContextMenuItems}
        />
      ) : (
        <ServicesTableContent
          sorted={sorted}
          sortDescriptor={sortDescriptor}
          onSortChange={(d) => setSortDescriptor({ column: String(d.column), direction: d.direction })}
          triggerUpdate={triggerUpdate}
          toggleAutoStart={toggleAutoStart}
        />
      )}

      {/* Pull Modal */}
      <PullDialog
        open={pull.open}
        unit={pull.unit}
        name={pull.name}
        image={pull.image}
        autoRestart={pull.autoRestart}
        onClose={() => setPull((p) => ({ ...p, open: false }))}
        onRestart={(unit) => doAction(unit, "restart")}
      />

      <ConfirmDialogs
        totalCount={totalCount}
        confirmBulkStop={confirmBulkStop}
        setConfirmBulkStop={setConfirmBulkStop}
        confirmAction={confirmAction}
        setConfirmAction={setConfirmAction}
        onConfirmBulkStop={() => doBulkAction("stop")}
        onConfirmAction={(a) => doAction(a.unit, a.action)}
      />

      {/* Config Drawer */}
      <ServiceConfigSheet
        draft={configDrawer}
        onChange={setConfigDrawer}
        onClose={() => setConfigDrawer(null)}
        onSave={saveConfig}
      />
    </PageShell>
  );
}
