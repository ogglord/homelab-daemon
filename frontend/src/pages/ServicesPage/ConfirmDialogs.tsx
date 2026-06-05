import { ModalContent, ModalHeader, ModalTitle, ModalBody, ModalClose, ModalFooter } from "@/components/ui/modal";
import { Button } from "@/components/ui/button";

export interface ConfirmAction {
  unit: string;
  action: string;
  label: string;
}

export interface ConfirmDialogsProps {
  totalCount: number;
  confirmBulkStop: boolean;
  setConfirmBulkStop: (v: boolean) => void;
  confirmAction: ConfirmAction | null;
  setConfirmAction: (v: ConfirmAction | null) => void;
  onConfirmBulkStop: () => void;
  onConfirmAction: (a: ConfirmAction) => void;
}

/**
 * Two confirmation modals: bulk "stop all services" and the per-service
 * action confirmation. Extracted from ServicesPage so the page focuses
 * on data flow rather than dialog markup.
 */
export function ConfirmDialogs({
  totalCount,
  confirmBulkStop,
  setConfirmBulkStop,
  confirmAction,
  setConfirmAction,
  onConfirmBulkStop,
  onConfirmAction,
}: ConfirmDialogsProps) {
  return (
    <>
      <ModalContent isOpen={confirmBulkStop} onOpenChange={setConfirmBulkStop}>
        <ModalHeader>
          <ModalTitle>Stop all services?</ModalTitle>
        </ModalHeader>
        <ModalBody>
          <p className="text-muted-fg text-sm">
            This will stop all {totalCount} tracked services (except homelab-dash).
            Services will stay stopped until you press <strong>Start All</strong> — systemd will
            not restart them automatically.
          </p>
        </ModalBody>
        <ModalFooter>
          <ModalClose>Cancel</ModalClose>
          <Button intent="danger" onPress={onConfirmBulkStop}>Stop All</Button>
        </ModalFooter>
      </ModalContent>

      <ModalContent isOpen={!!confirmAction} onOpenChange={() => setConfirmAction(null)}>
        <ModalHeader>
          <ModalTitle>
            {confirmAction?.action === "stop" ? "Stop service?" : `Run ${confirmAction?.action}?`}
          </ModalTitle>
        </ModalHeader>
        <ModalBody>
          <p className="text-muted-fg text-sm">
            {confirmAction?.action === "stop"
              ? `This will stop "${confirmAction?.label}". It will be unavailable until started again.`
              : `This will ${confirmAction?.action} "${confirmAction?.label}".`}
          </p>
        </ModalBody>
        <ModalFooter>
          <ModalClose>Cancel</ModalClose>
          <Button
            intent={confirmAction?.action === "stop" ? "danger" : "primary"}
            onPress={() => { if (confirmAction) onConfirmAction(confirmAction); }}
          >
            {confirmAction?.action === "stop" ? "Stop" : "Confirm"}
          </Button>
        </ModalFooter>
      </ModalContent>
    </>
  );
}
