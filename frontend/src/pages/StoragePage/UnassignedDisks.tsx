import { Server } from "lucide-react";
import { CardHeader, CardTitle } from "@/components/ui/card";
import type { StorageDisk } from "@/types";
import { fmtBytes } from "./format";

export interface UnassignedDisksProps {
  disks: StorageDisk[];
}

export function UnassignedDisks({ disks }: UnassignedDisksProps) {
  return (
    <div className="rounded-lg border p-4 flex flex-col">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Server className="w-5 h-5 text-slate-500" />
          Unassigned Disks
        </CardTitle>
      </CardHeader>
      {disks.length === 0 ? (
        <div className="py-12 text-center">
          <p className="text-muted-fg italic">No unassigned disks found.</p>
        </div>
      ) : (
        <div className="mt-4 space-y-2">
          {disks.map((d) => (
            <div
              key={d.path}
              className="flex items-center justify-between text-sm bg-muted/50 p-3 rounded-lg border border-dashed border-slate-300 dark:border-slate-800"
            >
              <div className="flex items-center gap-2.5">
                <Server className="w-4 h-4 text-muted-fg shrink-0" />
                <div className="flex flex-col">
                  <span className="text-xs font-semibold">{d.path}</span>
                  <span
                    className="text-[10px] text-muted-fg truncate max-w-[200px]"
                    title={d.friendly_name || d.model}
                  >
                    {d.friendly_name || d.model || "Unknown model"}
                  </span>
                </div>
              </div>
              <div className="flex gap-4 items-center">
                <span className="font-medium text-xs text-muted-fg">{fmtBytes(d.size)}</span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
