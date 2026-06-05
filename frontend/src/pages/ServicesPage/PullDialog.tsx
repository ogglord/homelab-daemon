import { useEffect, useState } from "react";
import { toast } from "sonner";
import { RefreshCw } from "lucide-react";
import { ModalContent, ModalHeader, ModalTitle, ModalBody } from "@/components/ui/modal";
import { StatusDot } from "@/components/ui/status-dot";

export interface PullDialogProps {
  open: boolean;
  unit: string;
  name: string;
  image: string;
  autoRestart: boolean;
  onClose: () => void;
  onRestart: (unit: string) => void;
}

/**
 * Streams `podman pull` output for a single service unit over its own
 * EventSource (separate from the app-wide /events stream). When the
 * backend emits `done`, optionally restarts the service.
 */
export function PullDialog({ open, unit, name, image, autoRestart, onClose, onRestart }: PullDialogProps) {
  const [lines, setLines] = useState<string[]>([]);
  const [status, setStatus] = useState<"idle" | "running" | "done" | "error">("idle");

  useEffect(() => {
    if (!open) return;
    setLines([]);
    setStatus("running");

    const source = new EventSource(`/api/services/${unit}/pull-stream`);

    source.addEventListener("log", (e: MessageEvent) => {
      setLines((prev) => [...prev.slice(-200), e.data]);
    });
    source.addEventListener("done", () => {
      setStatus("done");
      source.close();
      toast.success(`Pull complete: ${name}`);
      if (autoRestart) {
        onRestart(unit);
      }
    });
    source.addEventListener("pull-error", (e: MessageEvent) => {
      setStatus("error");
      source.close();
      toast.error(`Pull failed: ${e.data}`);
    });

    return () => {
      source.close();
    };
  }, [open, unit, name, autoRestart, onRestart]);

  return (
    <ModalContent isOpen={open} onOpenChange={onClose} size="3xl">
      <ModalHeader>
        <ModalTitle className="font-mono text-sm">
          Pull: {name}
          {status === "running" && <RefreshCw className="inline size-4 ml-2 animate-spin" />}
          {status === "done" && (
            <span className="ml-2 inline-flex items-center gap-1.5">
              <StatusDot intent="success" />
              Done{autoRestart ? " — restarting" : ""}
            </span>
          )}
          {status === "error" && (
            <span className="ml-2 inline-flex items-center gap-1.5">
              <StatusDot intent="danger" />
              Error
            </span>
          )}
        </ModalTitle>
      </ModalHeader>
      <ModalBody>
        <p className="text-xs text-muted-fg mb-2">{image}</p>
        <pre className="text-xs font-mono text-fg bg-bg p-4 rounded-md overflow-auto max-h-[50vh] whitespace-pre-wrap">
          {lines.join("\n") || (status === "running" ? "Starting pull..." : "")}
        </pre>
      </ModalBody>
    </ModalContent>
  );
}
