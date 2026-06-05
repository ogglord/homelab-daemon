import { useState, useEffect } from "react";
import { PageShell } from "@/components/data-table";
import { Button } from "@/components/ui/button";
import {
  Table, TableBody, TableCell, TableColumn, TableHeader, TableRow,
} from "@/components/ui/table";
import {
  ModalContent, ModalHeader, ModalTitle, ModalBody,
} from "@/components/ui/modal";
import { StatusDot } from "@/components/ui/status-dot";
import { toast } from "sonner";
import type { ServiceInfo } from "@/types";

function StateBadge({ state, subState }: { state: string; subState?: string }) {
  const isActive = state === "active";
  const isFailed = state === "failed";
  const tone = isActive ? "success" : isFailed ? "danger" : "secondary";
  const label = subState && subState !== "dead" && subState !== "running"
    ? `${state} (${subState})`
    : state;

  return (
    <span className="inline-flex items-center gap-1.5 text-sm capitalize">
      <StatusDot intent={tone} />
      {label}
    </span>
  );
}

export default function DiagnosticsPage() {
  const [services, setServices] = useState<ServiceInfo[] | null>(null);
  const [logs, setLogs] = useState({ open: false, unit: "", name: "", content: "" });

  const fetchDiagnostics = () => {
    fetch("/api/diagnostics")
      .then((r) => r.json())
      .then((data) => setServices(data))
      .catch(() => toast.error("Failed to fetch diagnostics"));
  };

  useEffect(() => {
    fetchDiagnostics();
    const interval = setInterval(fetchDiagnostics, 5000);
    return () => clearInterval(interval);
  }, []);

  async function showLogs(unit: string) {
    const name = unit.replace(/\.service$/, "");
    setLogs({ open: true, unit, name, content: "Loading logs..." });
    try {
      const resp = await fetch(`/api/diagnostics/${unit}/logs`);
      const data = await resp.json();
      setLogs((prev) => ({ ...prev, content: data.logs ?? "No logs available" }));
    } catch {
      setLogs((prev) => ({ ...prev, content: "Failed to load logs" }));
    }
  }

  return (
    <>
      <PageShell<ServiceInfo>
        data={services}
        title="Base System Diagnostics"
        description="Read-only status and logs monitoring for critical base homelab services."
        emptyMessage="No diagnostics services found"
      >
        <Table className="mt-4" aria-label="System Diagnostics">
          <TableHeader columns={[
            { id: "name", name: "Service", isRowHeader: true },
            { id: "status", name: "Status", className: "w-40" },
            { id: "description", name: "Description", className: "hidden md:table-cell" },
            { id: "logs", name: "", className: "w-24" },
          ] as Array<{ id: string; name: string; isRowHeader?: boolean; className?: string }>}>
            {(column) => (
              <TableColumn isRowHeader={column.id === "name"} className={column.className ?? ""}>
                {column.name}
              </TableColumn>
            )}
          </TableHeader>
          <TableBody items={services ?? []}>
            {(svc: ServiceInfo) => (
              <TableRow>
                <TableCell className="text-sm font-semibold">{svc.name}</TableCell>
                <TableCell>
                  <StateBadge state={svc.active_state} subState={svc.sub_state} />
                </TableCell>
                <TableCell className="text-sm text-muted-fg max-w-xs truncate hidden md:table-cell">
                  {svc.description || "\u2014"}
                </TableCell>
                <TableCell>
                  <div className="flex justify-end">
                    <Button intent="secondary" onPress={() => showLogs(svc.unit_name)}>
                      View Logs
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </PageShell>

      {/* Logs Modal */}
      <ModalContent isOpen={logs.open} onOpenChange={(o) => setLogs((p) => ({ ...p, open: o }))} size="3xl">
        <ModalHeader>
          <ModalTitle className="font-mono text-sm">Logs: {logs.name}</ModalTitle>
        </ModalHeader>
        <ModalBody>
          <pre className="text-xs font-mono text-fg bg-bg p-4 rounded-md overflow-auto max-h-[60vh] whitespace-pre-wrap">
            {logs.content}
          </pre>
        </ModalBody>
      </ModalContent>
    </>
  );
}
