import { useState } from "react";
import { Bug, Clipboard } from "lucide-react";
import { toast } from "sonner";
import { PageShell, useTable, RowActions, useRowContextMenu } from "@/components/data-table";
import { ModalContent, ModalHeader, ModalTitle, ModalBody } from "@/components/ui/modal";
import { Button } from "@/components/ui/button";
import { StatusDot } from "@/components/ui/status-dot";
import { Sheet, SheetBody, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { MenuItem } from "@/components/ui/menu";
import { ContextMenuItem } from "@/components/ui/context-menu";
import {
  Table, TableBody, TableCell, TableColumn, TableHeader, TableRow,
} from "@/components/ui/table";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { BugReportModal } from "@/components/bug-report-modal";
import { useOverview } from "@/hooks/use-overview";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import type { BackupStatus } from "@/types";

// ── Helpers ─────────────────────────────────────────────────────────────

function durationBetween(start?: string, end?: string): string {
  if (!start || !end) return "";
  const ms = new Date(end).getTime() - new Date(start).getTime();
  if (ms < 0) return "";
  const sec = Math.floor(ms / 1000);
  if (sec < 60) return `${sec}s`;
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}m ${sec % 60}s`;
  const hours = Math.floor(min / 60);
  return `${hours}h ${min % 60}m`;
}

function timeUntil(dateStr?: string) {
  if (!dateStr) return "\u2014";
  const diff = new Date(dateStr).getTime() - Date.now();
  if (diff < 0) return "\u2014";
  const min = Math.floor(diff / 60000);
  const hours = Math.floor(min / 60);
  if (hours > 24) return `in ${Math.floor(hours / 24)} days`;
  if (hours > 0) return `in ${hours}h ${min % 60}m`;
  return `in ${min}m`;
}

function timeSince(dateStr?: string) {
  if (!dateStr || dateStr === "Never") return "Never";
  const diff = Date.now() - new Date(dateStr).getTime();
  if (diff < 0) return "Just now";
  const min = Math.floor(diff / 60000);
  const hours = Math.floor(min / 60);
  if (hours > 24) return `${Math.floor(hours / 24)} days ago`;
  if (hours > 0) return `${hours}h ${min % 60}m ago`;
  return `${min}m ago`;
}

function StateBadge({ state, result, enabled }: { state: string; result: string; enabled?: boolean }) {
  const isActive = state === "active" || state === "activating";
  const isFailed = result === "failed";

  if (enabled && !isActive && !isFailed) {
    return (
      <span className="inline-flex items-center gap-1.5 text-sm">
        <StatusDot intent="success" />
        Enabled
      </span>
    );
  }

  const tone = isActive ? "success" : isFailed ? "danger" : "secondary";
  const label = state === "activating" ? "Running" : state || "inactive";
  return (
    <span className="inline-flex items-center gap-1.5 text-sm">
      <StatusDot intent={tone} />
      {label}
    </span>
  );
}

// ── Page ────────────────────────────────────────────────────────────────

// ── Inner table component (rendered inside PageShell so useRowContextMenu works) ──

function BackupsTableContent({ jobs, onEdit, onRun, onLogs }: {
  jobs: BackupStatus[];
  onEdit: (job: BackupStatus) => void;
  onRun: (name: string) => void;
  onLogs: (name: string) => void;
}) {
  const onRowCtx = useRowContextMenu<BackupStatus>();

  return (
    <Table className="mt-4" aria-label="Backup Jobs">
      <TableHeader columns={[
        { id: "name", name: "Name", isRowHeader: true, className: "w-[250px]" },
        { id: "lastRun", name: "Last Run", className: "w-[140px] hidden sm:table-cell" },
        { id: "nextRun", name: "Next Run", className: "w-[140px] hidden sm:table-cell" },
        { id: "requiresMount", name: "Requires Mount", className: "w-[180px] hidden lg:table-cell" },
        { id: "status", name: "Status", className: "w-[120px]" },
        { id: "result", name: "Result", className: "w-[120px] hidden md:table-cell" },
        { id: "actions", name: "", className: "w-[60px]" },
      ] as Array<{ id: string; name: string; isRowHeader?: boolean; className?: string }>}>
        {(column) => (
          <TableColumn isRowHeader={column.isRowHeader} className={column.className ?? ""}>
            {column.name}
          </TableColumn>
        )}
      </TableHeader>
      <TableBody items={jobs}>
        {(job: BackupStatus) => {
          const requiresMount = (job.requires_mount ?? []).join(", ") || "\u2014";
          return (
            <TableRow onContextMenu={onRowCtx(job)}>
              <TableCell className="text-sm">{job.unit}</TableCell>
              <TableCell className="text-sm text-muted-fg hidden sm:table-cell">
                {job.last_run_start && job.last_run_end ? (
                  <Tooltip>
                    <TooltipTrigger className="cursor-default">
                      {timeSince(job.last_run_start)}
                    </TooltipTrigger>
                    <TooltipContent>
                      Duration: {durationBetween(job.last_run_start, job.last_run_end)}
                    </TooltipContent>
                  </Tooltip>
                ) : (
                  timeSince(job.last_run_start)
                )}
              </TableCell>
              <TableCell className="text-sm text-muted-fg hidden sm:table-cell">
                {timeUntil(job.next_run)}
              </TableCell>
              <TableCell className="text-sm text-muted-fg hidden lg:table-cell">
                {requiresMount}
              </TableCell>
              <TableCell>
                <StateBadge state={job.active_state} result={job.result ?? ""} enabled={job.enabled ?? false} />
              </TableCell>
              <TableCell className="text-sm text-muted-fg hidden md:table-cell">
                {job.result ?? "\u2014"}
              </TableCell>
              <TableCell>
                <RowActions label={`Actions for ${job.unit}`}>
                  <MenuItem onAction={() => onEdit(job)}>Edit</MenuItem>
                  <MenuItem intent="danger" onAction={() => onRun(job.unit)}>Run now</MenuItem>
                  <MenuItem onAction={() => onLogs(job.unit)}>Logs</MenuItem>
                </RowActions>
              </TableCell>
            </TableRow>
          );
        }}
      </TableBody>
    </Table>
  );
}

// ── Page ────────────────────────────────────────────────────────────────

export default function BackupsPage() {
  const { data: jobs } = useOverview((o) => o?.Backups ?? null);
  const [loadingAction, setLoadingAction] = useState<string | null>(null);
  const [editingJob, setEditingJob] = useState<BackupStatus | null>(null);
  const [logs, setLogs] = useState<{ open: boolean; name: string; content: string }>({
    open: false, name: "", content: "",
  });
  const [isBugOpen, setIsBugOpen] = useState(false);
  const [bugService, setBugService] = useState("");

  const { filterText, setFilterText, filtered, sorted } = useTable<BackupStatus>({
    data: jobs ?? [],
    defaultSort: { column: "name", direction: "ascending" },
    filterField: "unit",
  });

  async function runJob(name: string) {
    setLoadingAction(name);
    try {
      const resp = await fetch(`/api/backups/${name}/run`, { method: "POST" });
      const data = await resp.json();
      if (data.success) {
        toast.success(`Backup "${name}" started`);
      } else {
        toast.error(data.error ?? "Failed to start backup");
      }
    } catch {
      toast.error("Failed to start backup");
    } finally {
      setLoadingAction(null);
    }
  }

  async function showLogs(name: string) {
    setLogs({ open: true, name, content: "Loading logs..." });
    try {
      const resp = await fetch(`/api/backups/${name}/logs`);
      const data = await resp.json();
      setLogs((prev) => ({ ...prev, content: data.logs ?? "No logs available" }));
    } catch {
      setLogs((prev) => ({ ...prev, content: "Failed to load logs" }));
    }
  }

  async function saveJob(e: React.FormEvent) {
    e.preventDefault();
    if (!editingJob) return;
    try {
      const resp = await fetch(`/api/backups/${editingJob.unit}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          enabled: editingJob.enabled,
          schedule: editingJob.schedule,
          healthcheck_uuid: editingJob.healthcheck_uuid,
          pause_service: editingJob.pause_service,
        }),
      });
      const data = await resp.json();
      if (data.success) {
        toast.success(`Backup "${editingJob.unit}" updated`);
        setEditingJob(null);
      } else {
        toast.error(data.error ?? "Failed to update backup");
      }
    } catch {
      toast.error("Failed to update backup");
    }
  }

  const handleCopy = async (content: string) => {
    try {
      await navigator.clipboard.writeText(content);
      toast.success("Logs copied to clipboard");
    } catch {
      toast.error("Failed to copy");
    }
  };

  return (
    <>
      <PageShell<BackupStatus>
        data={jobs}
        title="Backups"
        description={(n, f) => `${n} job${n !== 1 ? "s" : ""}${f ? ` matching "${f}"` : ""}`}
        searchable
        filter={filterText}
        onFilterChange={setFilterText}
        emptyMessage="No backup jobs found"
        emptyDescription="B2 backup systemd timers will appear here"
        contextMenu={(item) => (
          <>
            <ContextMenuItem onAction={() => setEditingJob(item)}>Edit</ContextMenuItem>
            <ContextMenuItem intent="danger" onAction={() => runJob(item.unit)}>Run now</ContextMenuItem>
            <ContextMenuItem onAction={() => showLogs(item.unit)}>Logs</ContextMenuItem>
          </>
        )}
      >
        <BackupsTableContent
          jobs={sorted}
          onEdit={setEditingJob}
          onRun={runJob}
          onLogs={showLogs}
        />
      </PageShell>

      {/* Logs Dialog */}
      <ModalContent isOpen={logs.open} onOpenChange={(o) => setLogs((p) => ({ ...p, open: o }))} className="sm:max-w-[85%] min-w-[600px]">
        <ModalHeader>
          <div className="flex items-center justify-between w-full">
            <ModalTitle className="font-mono text-sm">Logs: {logs.name}</ModalTitle>
            <div className="flex gap-2 items-center">
              <button
                onClick={() => { setBugService(logs.name); setIsBugOpen(true); }}
                className="text-muted-fg hover:text-fg transition-colors p-1 rounded-md"
                title="Report Bug with these logs"
              >
                <Bug className="size-5" />
              </button>
              <Button intent="secondary" size="sm" onPress={() => handleCopy(logs.content)} aria-label="Copy logs to clipboard">
                <Clipboard className="w-4 h-4 mr-1" />
                Copy
              </Button>
            </div>
          </div>
        </ModalHeader>
        <ModalBody>
          <pre className="text-xs font-mono text-fg bg-bg p-4 rounded-md overflow-auto max-h-[65vh] whitespace-pre-wrap">
            {logs.content}
          </pre>
        </ModalBody>
      </ModalContent>

      {/* Edit Drawer */}
      <Sheet isOpen={editingJob !== null} onOpenChange={(open) => !open && setEditingJob(null)}>
        <SheetContent side="right" className="sm:max-w-md">
          {editingJob && (
            <form onSubmit={saveJob} className="flex flex-col h-full">
              <SheetHeader>
                <SheetTitle>Configure {editingJob.unit}</SheetTitle>
              </SheetHeader>
              <SheetBody className="space-y-6 flex-1 overflow-y-auto">
                <div className="space-y-4">
                  <Checkbox
                    isSelected={editingJob.enabled}
                    onChange={(checked) => setEditingJob((p) => ({ ...p!, enabled: checked }))}
                  >
                    Enabled
                  </Checkbox>

                  <div className="space-y-2">
                    <label className="text-sm font-medium">Schedule (Cron format)</label>
                    <Input
                      value={editingJob.schedule ?? ""}
                      onChange={(e) => setEditingJob((p) => ({ ...p!, schedule: e.target.value }))}
                      placeholder="0 4 * * *"
                    />
                  </div>

                  <div className="space-y-2">
                    <label className="text-sm font-medium">Healthcheck.io UUID (Optional)</label>
                    <Input
                      value={editingJob.healthcheck_uuid ?? ""}
                      onChange={(e) => setEditingJob((p) => ({ ...p!, healthcheck_uuid: e.target.value }))}
                      placeholder="e.g. 5bb56709-ec9b-..."
                    />
                  </div>

                  <div className="space-y-2">
                    <label className="text-sm font-medium">Pause Service (Optional)</label>
                    <Input
                      value={editingJob.pause_service ?? ""}
                      onChange={(e) => setEditingJob((p) => ({ ...p!, pause_service: e.target.value }))}
                      placeholder="e.g. postgresql.service"
                    />
                  </div>
                </div>

                <div className="flex justify-end gap-3 pt-4 border-t border-border mt-auto">
                  <Button intent="secondary" onPress={() => setEditingJob(null)}>Cancel</Button>
                  <Button type="submit" intent="primary">Save Changes</Button>
                </div>
              </SheetBody>
            </form>
          )}
        </SheetContent>
      </Sheet>

      <BugReportModal
        isOpen={isBugOpen}
        onOpenChange={setIsBugOpen}
        defaultService={bugService}
        defaultLogs={logs.content}
      />
    </>
  );
}
