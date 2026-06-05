import { Sheet, SheetBody, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export interface ServiceConfigDraft {
  open: boolean;
  unit: string;
  name: string;
  boot_order: number;
  boot_delay: number;
  restart_delay: number;
  depends_on: string;
}

export interface ServiceConfigSheetProps {
  draft: ServiceConfigDraft | null;
  onChange: (draft: ServiceConfigDraft) => void;
  onClose: () => void;
  onSave: () => void;
}

/**
 * Right-edge drawer for editing a service's daemon-side scheduling
 * configuration (boot order/delay, restart delay, dependency list).
 * The page owns the draft state and persistence; this component is a
 * controlled form.
 */
export function ServiceConfigSheet({ draft, onChange, onClose, onSave }: ServiceConfigSheetProps) {
  return (
    <Sheet isOpen={!!draft} onOpenChange={(open) => !open && onClose()}>
      <SheetContent side="right" className="sm:max-w-md">
        {draft && (
          <>
            <SheetHeader>
              <SheetTitle>Configure {draft.name}</SheetTitle>
            </SheetHeader>
            <SheetBody className="space-y-6">
              <div className="space-y-2">
                <label className="text-sm font-medium">Boot Order</label>
                <Input
                  type="number"
                  value={draft.boot_order}
                  onChange={(e) => onChange({ ...draft, boot_order: Number(e.target.value) })}
                />
                <span className="text-xs text-muted-fg block">Lower numbers start first (e.g. Caddy has order 1)</span>
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">Boot Delay (seconds)</label>
                <Input
                  type="number"
                  value={draft.boot_delay}
                  onChange={(e) => onChange({ ...draft, boot_delay: Number(e.target.value) })}
                />
                <span className="text-xs text-muted-fg block">Time to wait after dependencies are running</span>
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">Restart Delay (seconds)</label>
                <Input
                  type="number"
                  value={draft.restart_delay}
                  onChange={(e) => onChange({ ...draft, restart_delay: Number(e.target.value) })}
                />
                <span className="text-xs text-muted-fg block">Delay before restarting crashed/failed services</span>
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">Dependencies (Depends On)</label>
                <Input
                  type="text"
                  value={draft.depends_on}
                  onChange={(e) => onChange({ ...draft, depends_on: e.target.value })}
                  placeholder="e.g. postgresql.service, caddy.service"
                />
                <span className="text-xs text-muted-fg block">Comma-separated list of systemd service names</span>
              </div>

              <div className="flex justify-end gap-3 pt-4 border-t border-border">
                <Button intent="secondary" onPress={onClose}>Cancel</Button>
                <Button intent="primary" onPress={onSave}>Save Changes</Button>
              </div>
            </SheetBody>
          </>
        )}
      </SheetContent>
    </Sheet>
  );
}
