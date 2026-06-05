import { useState } from "react";
import { Play, Square, Pause, RefreshCw, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { PageShell, RowActions } from "@/components/data-table";
import { Button } from "@/components/ui/button";
import {
  ModalContent, ModalHeader, ModalTitle, ModalBody, ModalFooter, ModalClose,
} from "@/components/ui/modal";
import {
  MenuItem,
} from "@/components/ui/menu";
import {
  Table, TableBody, TableCell, TableColumn, TableHeader, TableRow,
} from "@/components/ui/table";
import { StatusDot } from "@/components/ui/status-dot";
import { useOverview } from "@/hooks/use-overview";
import type { VMInfo } from "@/types";

function StateBadge({ state }: { state: string }) {
  const isRunning = state === "Running";
  const isPaused = state === "Paused";
  const isOff = state === "Shut Off";
  const tone = isRunning ? "success" : isPaused ? "warning" : isOff ? "secondary" : "danger";
  return (
    <span className="inline-flex items-center gap-1.5 text-sm">
      <StatusDot intent={tone} />
      {state}
    </span>
  );
}

export default function VMsPage() {
  const overview = useOverview((o) => o?.VMs ?? []);
  const vms = overview.data ?? [];
  const [loadingAction, setLoadingAction] = useState<string | null>(null);
  const [confirmDestroy, setConfirmDestroy] = useState<string | null>(null);

  async function doAction(name: string, action: string) {
    setConfirmDestroy(null);
    setLoadingAction(`${name}:${action}`);
    try {
      const resp = await fetch(`/api/vms/${name}/${action}`, { method: "POST" });
      const data = await resp.json();
      if (data.success) {
        toast.success(data.message ?? `${action} succeeded`);
      } else {
        toast.error(data.error ?? "Action failed");
      }
    } catch {
      toast.error("Action failed");
    } finally {
      setLoadingAction(null);
    }
  }

  return (
    <>
      <PageShell<VMInfo>
        data={vms}
        title="Virtual Machines"
        description={(n) => `${n} VM${n !== 1 ? "s" : ""}`}
        emptyMessage="No virtual machines found"
        emptyDescription="VMs managed by libvirt will appear here"
      >
        <Table aria-label="Virtual Machines">
          <TableHeader columns={[
            { id: "name", name: "Name", isRowHeader: true },
            { id: "state", name: "State", className: "w-36" },
            { id: "memory", name: "Memory", className: "hidden sm:table-cell w-28" },
            { id: "cpus", name: "vCPUs", className: "hidden sm:table-cell w-20" },
            { id: "actions", name: "", className: "w-10" },
          ] as Array<{ id: string; name: string; isRowHeader?: boolean; className?: string }>}>
            {(column) => (
              <TableColumn
                isRowHeader={column.id === "name"}
                className={column.className ?? ""}
              >
                {column.name}
              </TableColumn>
            )}
          </TableHeader>
          <TableBody items={vms ?? []}>
            {(vm: VMInfo) => (
              <TableRow>
                <TableCell className="text-sm">{vm.name}</TableCell>
                <TableCell><StateBadge state={vm.state} /></TableCell>
                <TableCell className="text-sm text-muted-fg hidden sm:table-cell">{vm.memory}</TableCell>
                <TableCell className="text-sm text-muted-fg hidden sm:table-cell">{vm.cpus}</TableCell>
                <TableCell>
                  <RowActions label={`Actions for ${vm.name}`}>
                    {vm.state !== "Running" && (
                      <MenuItem onAction={() => doAction(vm.name, "start")}>
                        <Play className="size-4 text-success" />
                        Start
                      </MenuItem>
                    )}
                    {vm.state === "Running" && (
                      <>
                        <MenuItem onAction={() => doAction(vm.name, "shutdown")}>
                          <Square className="size-4 text-warning" />
                          Shutdown
                        </MenuItem>
                        <MenuItem onAction={() => doAction(vm.name, "suspend")}>
                          <Pause className="size-4 text-warning" />
                          Suspend
                        </MenuItem>
                      </>
                    )}
                    {vm.state === "Paused" && (
                      <MenuItem onAction={() => doAction(vm.name, "resume")}>
                        <RefreshCw className="size-4 text-primary" />
                        Resume
                      </MenuItem>
                    )}
                    <MenuItem onAction={() => setConfirmDestroy(vm.name)}>
                      <Trash2 className="size-4 text-danger" />
                      Destroy
                    </MenuItem>
                  </RowActions>
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </PageShell>

      {/* Destroy Confirmation Dialog */}
      <ModalContent isOpen={!!confirmDestroy} onOpenChange={() => setConfirmDestroy(null)}>
        <ModalHeader>
          <ModalTitle>Destroy VM?</ModalTitle>
        </ModalHeader>
        <ModalBody>
          <p className="text-muted-fg text-sm">
            This will forcefully destroy "{confirmDestroy}". All unsaved data in the VM will be lost.
            This action cannot be undone.
          </p>
        </ModalBody>
        <ModalFooter>
          <ModalClose>Cancel</ModalClose>
          <Button
            intent="danger"
            onPress={() => { if (confirmDestroy) doAction(confirmDestroy, "destroy"); }}
          >
            Destroy
          </Button>
        </ModalFooter>
      </ModalContent>
    </>
  );
}
