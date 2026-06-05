import { useState } from "react";
import { Bug } from "lucide-react";
import { ModalContent, ModalHeader, ModalTitle, ModalBody } from "@/components/ui/modal";
import { BugReportModal } from "@/components/bug-report-modal";

export interface LogsDialogProps {
  open: boolean;
  name: string;
  content: string;
  onClose: () => void;
}

/**
 * Read-only service logs viewer. Exposes a bug-report shortcut that
 * pre-fills the report with the visible logs.
 */
export function LogsDialog({ open, name, content, onClose }: LogsDialogProps) {
  const [isBugOpen, setIsBugOpen] = useState(false);

  return (
    <>
      <ModalContent isOpen={open} onOpenChange={onClose} size="3xl">
        <ModalHeader className="flex justify-between items-center pr-8">
          <ModalTitle className="font-mono text-sm">Logs: {name}</ModalTitle>
          <button
            onClick={() => setIsBugOpen(true)}
            className="text-muted-fg hover:text-fg transition-colors p-1 rounded-md"
            title="Report Bug with these logs"
          >
            <Bug className="size-5" />
          </button>
        </ModalHeader>
        <ModalBody>
          <pre className="text-xs font-mono text-fg bg-bg p-4 rounded-md overflow-auto max-h-[60vh] whitespace-pre-wrap">
            {content}
          </pre>
        </ModalBody>
      </ModalContent>
      <BugReportModal
        isOpen={isBugOpen}
        onOpenChange={setIsBugOpen}
        defaultService={name}
        defaultLogs={content}
      />
    </>
  );
}
