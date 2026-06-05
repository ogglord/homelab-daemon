import { useState, useEffect, useCallback } from "react";
import { AlertTriangle, RefreshCw } from "lucide-react";
import { CardHeader, CardTitle, CardDescription, Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableColumn, TableHeader, TableRow } from "@/components/ui/table";
import { Input } from "@/components/ui/input";
import { ModalContent, ModalHeader, ModalTitle, ModalBody, ModalFooter, ModalClose } from "@/components/ui/modal";
import { toast } from "sonner";
import { StatusDot } from "@/components/ui/status-dot";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

interface Secret {
  name: string;
  description: string;
  services: string[];
  runPath: string;
  present: boolean;
  modified_at?: string;
  preview?: string;
}

function RelativeTime({ iso }: { iso?: string }) {
  if (!iso) return <span className="text-muted-fg text-xs">\u2014</span>;
  const date = new Date(iso);
  const diff = Date.now() - date.getTime();
  const mins = Math.floor(diff / 60_000);
  const hours = Math.floor(diff / 3_600_000);
  const days = Math.floor(diff / 86_400_000);
  let label: string;
  if (mins < 1) label = "just now";
  else if (mins < 60) label = `${mins}m ago`;
  else if (hours < 24) label = `${hours}h ago`;
  else label = `${days}d ago`;
  return (
    <Tooltip>
      <TooltipTrigger className="text-xs text-muted-fg tabular-nums">{label}</TooltipTrigger>
      <TooltipContent>{date.toLocaleString()}</TooltipContent>
    </Tooltip>
  );
}

export default function SecretsPage() {
  const [secrets, setSecrets] = useState<Secret[]>([]);
  const [deployPending, setDeployPending] = useState(false);
  const [loading, setLoading] = useState(true);
  const [deploying, setDeploying] = useState(false);
  const [editModal, setEditModal] = useState<{ open: boolean; name: string; description: string; value: string }>(
    { open: false, name: "", description: "", value: "" }
  );

  const fetchSecrets = useCallback(async () => {
    try {
      const resp = await fetch("/api/secrets");
      const data = await resp.json();
      setSecrets(data.secrets || []);
      setDeployPending(data.deploy_pending || false);
    } catch {
      toast.error("Failed to load secrets");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchSecrets(); }, [fetchSecrets]);

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editModal.value.trim()) return;
    try {
      const resp = await fetch(`/api/secrets/${encodeURIComponent(editModal.name)}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ value: editModal.value }),
      });
      const data = await resp.json();
      if (resp.ok) {
        toast.success(`Secret ${editModal.name} updated`);
        setEditModal({ ...editModal, open: false, value: "" });
        fetchSecrets();
      } else {
        toast.error(data.error || "Failed to update secret");
      }
    } catch {
      toast.error("Failed to update secret");
    }
  };

  const handleDeploy = async () => {
    setDeploying(true);
    try {
      const resp = await fetch("/api/secrets/deploy", { method: "POST" });
      if (!resp.ok) { toast.error("Deploy failed to start"); setDeploying(false); return; }
      toast.info("Deploy started \u2014 activating system configuration\u2026");
      const reader = resp.body?.getReader();
      if (!reader) return;
      const decoder = new TextDecoder();
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        const chunk = decoder.decode(value);
        if (chunk.includes('"type":"done"')) {
          toast.success("Deploy completed successfully!");
          setDeployPending(false);
          fetchSecrets();
          break;
        } else if (chunk.includes('"type":"error"')) {
          toast.error("Deploy failed!");
          break;
        }
      }
    } catch {
      toast.error("Deploy encountered an error");
    } finally {
      setDeploying(false);
    }
  };

  if (loading) {
    return <div className="flex items-center justify-center h-full text-muted-fg text-sm">Loading secrets\u2026</div>;
  }

  return (
    <div className="flex flex-col gap-6 p-4 md:p-6 lg:p-8 max-w-7xl mx-auto w-full">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold tracking-tight text-fg">Secrets</h1>
        <div className="flex gap-2">
          <Button intent="outline" onPress={fetchSecrets}>
            <RefreshCw className="w-4 h-4 mr-2" />
            Refresh
          </Button>
        </div>
      </div>

      {deployPending && (
        <Card className="bg-yellow-500/10 border-yellow-500/20 dark:bg-yellow-500/10 dark:border-yellow-500/20">
          <CardHeader className="flex flex-row items-center gap-4 py-4">
            <AlertTriangle className="w-8 h-8 text-yellow-600 dark:text-yellow-500 shrink-0" />
            <div className="flex-1">
              <CardTitle className="text-yellow-700 dark:text-yellow-500">Deploy Pending</CardTitle>
              <CardDescription className="text-yellow-600/80 dark:text-yellow-500/80">
                Secrets have been modified. You must deploy the configuration to activate them.
              </CardDescription>
            </div>
            <Button
              intent="danger"
              className="bg-yellow-600 hover:bg-yellow-700 text-white"
              onPress={handleDeploy}
              isPending={deploying}
            >
              Deploy Now
            </Button>
          </CardHeader>
        </Card>
      )}

      <Card>
        <Table aria-label="Secrets list">
          <TableHeader>
            <TableColumn isRowHeader>Name</TableColumn>
            <TableColumn>Description</TableColumn>
            <TableColumn>Services</TableColumn>
            <TableColumn>Value preview</TableColumn>
            <TableColumn>Last rotated</TableColumn>
            <TableColumn className="w-10" />
          </TableHeader>
          <TableBody items={secrets}>
            {(item: Secret) => (
              <TableRow id={item.name}>
                <TableCell className="font-mono text-xs">{item.name}</TableCell>
                <TableCell className="text-muted-fg text-sm">{item.description}</TableCell>
                <TableCell>
                  <div className="flex flex-wrap gap-1">
                    {item.services?.map((svc) => (
                      <span key={svc} className="px-1.5 py-0.5 rounded text-[10px] font-mono bg-muted text-muted-fg">
                        {svc}
                      </span>
                    ))}
                    {!item.services?.length && <span className="text-muted-fg text-xs">\u2014</span>}
                  </div>
                </TableCell>
                <TableCell>
                  {item.present ? (
                    <Tooltip>
                      <TooltipTrigger className="flex items-center gap-1.5 cursor-default">
                        <StatusDot intent="success" />
                        <span className="font-mono text-xs tracking-widest">
                          {item.preview ?? "\u2022\u2022\u2022"}
                        </span>
                      </TooltipTrigger>
                      <TooltipContent>First 3 characters of decrypted secret</TooltipContent>
                    </Tooltip>
                  ) : (
                    <Tooltip>
                      <TooltipTrigger className="flex items-center gap-1.5 cursor-default">
                        <StatusDot intent="danger" />
                        <span className="text-muted-fg text-xs">Missing</span>
                      </TooltipTrigger>
                      <TooltipContent>Secret file is absent or empty</TooltipContent>
                    </Tooltip>
                  )}
                </TableCell>
                <TableCell>
                  <RelativeTime iso={item.modified_at} />
                </TableCell>
                <TableCell>
                  <Button
                    intent="outline"
                    size="sm"
                    onPress={() => setEditModal({ open: true, name: item.name, description: item.description, value: "" })}
                  >
                    Edit
                  </Button>
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>

      {/* Edit Modal */}
      <ModalContent isOpen={editModal.open} onOpenChange={(open) => !open && setEditModal({ ...editModal, open: false })}>
        <form onSubmit={handleSave}>
          <ModalHeader>
            <ModalTitle>Edit Secret</ModalTitle>
          </ModalHeader>
          <ModalBody className="flex flex-col gap-4">
            <div>
              <p className="text-sm font-medium">Name</p>
              <p className="text-sm font-mono text-muted-fg">{editModal.name}</p>
            </div>
            <div>
              <p className="text-sm font-medium">Description</p>
              <p className="text-sm text-muted-fg">{editModal.description}</p>
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium" htmlFor="secret-val">New Value</label>
              <Input
                id="secret-val"
                type="password"
                placeholder="Enter secret value\u2026"
                value={editModal.value}
                onChange={(e) => setEditModal({ ...editModal, value: e.target.value })}
                autoFocus
              />
            </div>
          </ModalBody>
          <ModalFooter>
            <ModalClose>Cancel</ModalClose>
            <Button type="submit" isDisabled={!editModal.value.trim()}>
              Save changes
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </div>
  );
}
