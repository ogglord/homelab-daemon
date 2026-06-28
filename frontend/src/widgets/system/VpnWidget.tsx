import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useOverview } from "@/hooks/use-overview";
import { ShieldCheck, ShieldOff } from "lucide-react";

export function VpnWidget() {
  const { data: vpn } = useOverview((o) => o?.VPN);

  if (!vpn) {
    return (
      <Skeleton isLoading>
        <Card className="h-full"><CardContent>{"."}</CardContent></Card>
      </Skeleton>
    );
  }

  if (!vpn.Enabled) {
    return (
      <Card className="h-full">
        <CardContent>
          <div className="flex items-center gap-1.5 mb-2">
            <ShieldOff className="h-3.5 w-3.5 text-muted-fg" />
            <span className="text-[10px] tracking-[0.15em] uppercase text-muted-fg font-semibold">VPN</span>
          </div>
          <div className="text-xs text-muted-fg">Disabled</div>
        </CardContent>
      </Card>
    );
  }

  const Icon = vpn.Connected ? ShieldCheck : ShieldOff;
  const dot = vpn.Connected ? "bg-success" : "bg-destructive";

  return (
    <Card className="h-full [--gutter:--spacing(3)]">
      <CardContent>
        <div className="flex items-center gap-1.5 mb-2">
          <Icon className={`h-3.5 w-3.5 ${vpn.Connected ? "text-primary" : "text-destructive"}`} />
          <span className="text-[10px] tracking-[0.15em] uppercase text-primary/60 font-semibold">VPN</span>
          <span className={`ml-auto h-2 w-2 rounded-full ${dot}`} />
        </div>

        <div className="text-xs font-mono font-bold text-primary mb-1">
          {vpn.Provider} · {vpn.Type}
        </div>
        <div className="grid grid-cols-2 gap-y-1 text-[10px]">
          <span className="text-muted-fg">Public IP</span>
          <span className="font-mono text-right">{vpn.PublicIP || "—"}</span>
          <span className="text-muted-fg">Fwd port</span>
          <span className="font-mono text-right">{vpn.ForwardedPort || "—"}</span>
          <span className="text-muted-fg">Country</span>
          <span className="font-mono text-right">{vpn.ServerCountry || "—"}</span>
        </div>
        {vpn.ErrMsg && <div className="mt-1 text-[10px] text-destructive truncate">{vpn.ErrMsg}</div>}
      </CardContent>
    </Card>
  );
}
