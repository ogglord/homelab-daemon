import { useState, useEffect } from "react";
import { Button } from "@/components/ui/button";
import {
  Modal,
  ModalContent,
  ModalHeader,
  ModalTitle,
  ModalBody,
  ModalFooter,
  ModalClose,
} from "@/components/ui/modal";
import { Checkbox } from "@/components/ui/checkbox";
import { toast } from "sonner";
import { Bug } from "lucide-react";

interface BugReportModalProps {
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  defaultService?: string;
  defaultLogs?: string;
}

export function BugReportModal({ isOpen, onOpenChange, defaultService, defaultLogs }: BugReportModalProps) {
  const [description, setDescription] = useState("");
  const [issueOnly, setIssueOnly] = useState(false);
  const [model, setModel] = useState("");
  const [models, setModels] = useState<string[]>([]);
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    if (isOpen) {
      fetch("/api/models")
        .then(res => res.text())
        .then(text => {
          const lines = text.split("\n")
            .map(l => l.trim())
            .filter(Boolean)
            .filter(l => !l.startsWith("provider") && !l.startsWith("Warning:"));
            
          const parsedModels = lines.map(l => {
            const parts = l.split(/\s+/);
            if (parts.length >= 2) {
              return parts[1];
            }
            return l;
          });
          setModels(parsedModels);
        })
        .catch(console.error);
    }
  }, [isOpen]);

  const handleSubmit = async () => {
    if (!description.trim()) {
      toast.error("Please enter a description");
      return;
    }

    setIsSubmitting(true);
    try {
      const res = await fetch("/api/bug", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          description,
          issue_only: issueOnly,
          model,
          service: defaultService,
          logs: defaultLogs,
        }),
      });

      const { id } = await res.json();

      onOpenChange(false);
      setDescription("");

      // Start SSE listener for this task
      const evtSource = new EventSource(`/api/bug/status/${id}`);
      let toastId = toast.loading("Agent is initializing...");

      evtSource.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);

          // Our CLI stage events (prerequisites, worktree, etc.)
          if (data.type === 'stage') {
            toast.loading(data.message, { id: toastId });
          } else if (data.type === 'error') {
            toast.error(data.message, { id: toastId });
            evtSource.close();
            return;
          }

          // pi RPC events
          switch (data.type) {
            case 'agent_start':
              toast.loading('Agent is analyzing...', { id: toastId });
              break;
            case 'tool_execution_start':
              if (data.tool) toast.loading(`Running ${data.tool}...`, { id: toastId });
              break;
            case 'agent_end':
              // agent_end carries final messages in data.messages
              const msgs = Array.isArray(data.messages) ? data.messages : [];
              const hasError = msgs.some((m: any) =>
                m?.content?.some?.((c: any) => c?.text?.includes?.('error') || c?.text?.includes?.('failed'))
              );
              if (hasError) {
                toast.error('Agent completed with errors', { id: toastId });
              } else {
                toast.success('Agent completed successfully', { id: toastId });
              }
              evtSource.close();
              break;
            case 'extension_error':
              toast.error(`Extension error: ${data.message || 'unknown'}`, { id: toastId });
              break;
          }

          // Also handle our old log format (from CLI emit)
          if (data.type === 'log') {
            if (data.message?.includes('successfully') || data.message?.includes('PR created')) {
              toast.success(data.message, { id: toastId });
              evtSource.close();
            } else if (data.message?.includes('exited with error')) {
              toast.error(data.message, { id: toastId });
              evtSource.close();
            }
          }
        } catch (e) {
          console.error("Failed to parse SSE event", e);
        }
      };

      evtSource.onerror = () => {
        evtSource.close();
      };

    } catch (err: any) {
      toast.error("Failed to submit bug report: " + err.message);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Modal isOpen={isOpen} onOpenChange={onOpenChange}>
      <ModalContent className="sm:max-w-md">
        <ModalHeader>
          <ModalTitle className="flex items-center gap-2">
            <Bug className="size-5 text-muted-fg" />
            Report Bug
          </ModalTitle>
        </ModalHeader>
        <ModalBody className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-1">Description</label>
            <textarea
              className="w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm placeholder:text-muted-fg focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring min-h-[100px]"
              placeholder="What went wrong?"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>

          <div className="flex items-center space-x-2">
            <Checkbox isSelected={issueOnly} onChange={setIssueOnly}>
              Create Issue Only (no automated PR)
            </Checkbox>
          </div>

          {models.length > 0 && (
            <div>
              <label className="block text-sm font-medium mb-1">Model (optional)</label>
              <select
                className="w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                value={model}
                onChange={(e) => setModel(e.target.value)}
              >
                <option value="">Default model</option>
                {models.map(m => (
                  <option key={m} value={m}>{m}</option>
                ))}
              </select>
            </div>
          )}

          {defaultService && (
            <div className="text-xs text-muted-fg">
              Context from <strong>{defaultService}</strong> logs will be included automatically.
            </div>
          )}
        </ModalBody>
        <ModalFooter>
          <ModalClose intent="secondary" isDisabled={isSubmitting}>Cancel</ModalClose>
          <Button intent="primary" onPress={handleSubmit} isPending={isSubmitting}>
            Submit Report
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
}
