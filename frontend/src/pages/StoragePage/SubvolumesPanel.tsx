import { FolderPlus, Trash2 } from "lucide-react";
import { CardHeader, CardTitle, CardAction } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Table, TableHeader, TableRow, TableColumn, TableBody, TableCell } from "@/components/ui/table";
import type { StorageStatus } from "@/types";
import { fmtBytes } from "./format";

export interface SubvolumesPanelProps {
  status: StorageStatus | null;
  subvolumeSizes: Map<string, number>;
  calculatingUsage: boolean;
  actionLoading: boolean;
  onCalculateUsage: () => void;
  onCreate: () => void;
  onDelete: (path: string) => void;
}

export function SubvolumesPanel({
  status, subvolumeSizes, calculatingUsage, actionLoading,
  onCalculateUsage, onCreate, onDelete,
}: SubvolumesPanelProps) {
  const subvols = status?.subvolumes ?? [];
  const noMountedPool = (status?.pools || []).every((p) => p.state !== "mounted");

  return (
    <div className="rounded-lg border p-4 flex flex-col">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <FolderPlus className="w-5 h-5 text-emerald-500" />
          Subvolumes
        </CardTitle>
        <CardAction>
          <div className="flex items-center gap-2">
            <Button intent="secondary" size="xs" onPress={onCalculateUsage}
              isDisabled={calculatingUsage || subvols.length === 0}>
              {calculatingUsage ? "Calculating..." : "Calculate Usage"}
            </Button>
            <Button intent="primary" size="xs" onPress={onCreate}
              isDisabled={actionLoading || noMountedPool}>
              Create Subvolume
            </Button>
          </div>
        </CardAction>
      </CardHeader>
      {subvols.length === 0 ? (
        <div className="py-12 text-center">
          <p className="text-muted-fg italic">No subvolumes detected. Ensure a pool is mounted to list subvolumes.</p>
        </div>
      ) : (
        <Table className="mt-4" aria-label="Subvolumes">
          <TableHeader>
            <TableColumn isRowHeader>ID</TableColumn>
            <TableColumn>Name</TableColumn>
            <TableColumn>Usage</TableColumn>
            <TableColumn>Actions</TableColumn>
          </TableHeader>
          <TableBody>
            {subvols.map((sv) => {
              const size = subvolumeSizes.get(sv.path) ?? sv.used_bytes;
              const sizeText = size === -1
                ? (calculatingUsage
                    ? <span className="text-[11px] text-muted-fg animate-pulse">Calculating...</span>
                    : <span className="text-[11px] text-muted-fg italic">Not calculated</span>)
                : fmtBytes(size);
              return (
                <TableRow id={sv.id} key={sv.id}>
                  <TableCell className="text-xs">{sv.id}</TableCell>
                  <TableCell className="font-medium text-xs max-w-[200px] truncate">{sv.path.replace(/^\/pool\//, "")}</TableCell>
                  <TableCell className="text-xs font-medium">{sizeText}</TableCell>
                  <TableCell>
                    <Button intent="danger" size="xs" onPress={() => onDelete(sv.path)} isDisabled={actionLoading}>
                      <Trash2 className="w-3.5 h-3.5" />
                    </Button>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      )}
    </div>
  );
}
