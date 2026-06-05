import { MenuItem } from "@/components/ui/menu";
import { ContextMenuItem } from "@/components/ui/context-menu";

export interface ServiceActionItem {
  unit_name: string;
  name: string;
  state: string;
  type: string;
  daemon_enabled: boolean;
  image?: string;
  boot_order: number;
  boot_delay: number;
  restart_delay: number;
  depends_on?: string[];
}

export interface ServiceActionsCallbacks {
  onStopConfirm: (unit: string, name: string) => void;
  onStopAndDisable: (unit: string, name: string) => void;
  onRestart: (unit: string) => void;
  onStart: (unit: string) => void;
  onEnableAutoStart: (unit: string) => void;
  onShowLogs: (unit: string) => void;
  onConfigure: (item: ServiceActionItem) => void;
  onPull: (unit: string, image: string) => void;
  /**
   * Optional hook fired after every action so the context-menu can close
   * itself. The menu component doesn't get an action callback for free.
   */
  afterAction?: () => void;
}

function buildEntries(item: ServiceActionItem, cb: ServiceActionsCallbacks) {
  const after = () => cb.afterAction?.();
  return {
    isRunning: item.state === "active" || item.state === "activating",
    isDocker: item.type === "Docker",
    after,
    stop: () => { cb.onStopConfirm(item.unit_name, item.name); after(); },
    stopDisable: () => { cb.onStopAndDisable(item.unit_name, item.name); after(); },
    restart: () => { cb.onRestart(item.unit_name); after(); },
    start: () => { cb.onStart(item.unit_name); after(); },
    enable: () => { cb.onEnableAutoStart(item.unit_name); after(); },
    showLogs: () => { cb.onShowLogs(item.unit_name); after(); },
    configure: () => { cb.onConfigure(item); after(); },
    pull: () => { cb.onPull(item.unit_name, item.image ?? ""); after(); },
  };
}

/**
 * Action list rendered inside the dropdown menu attached to each row.
 */
export function renderMenuItems(item: ServiceActionItem, cb: ServiceActionsCallbacks) {
  const e = buildEntries(item, cb);
  return (
    <>
      {e.isRunning ? (
        <>
          <MenuItem intent="danger" onAction={e.stop}>Stop</MenuItem>
          <MenuItem intent="danger" onAction={e.stopDisable}>Stop &amp; Disable</MenuItem>
          <MenuItem intent="danger" onAction={e.restart}>Restart</MenuItem>
        </>
      ) : (
        <>
          <MenuItem onAction={e.start}>Start</MenuItem>
          {!item.daemon_enabled && (
            <MenuItem onAction={e.enable}>Enable (auto-start)</MenuItem>
          )}
        </>
      )}
      <MenuItem onAction={e.showLogs}>Logs</MenuItem>
      <MenuItem onAction={e.configure}>Configure</MenuItem>
      {e.isDocker && <MenuItem onAction={e.pull}>Pull</MenuItem>}
    </>
  );
}

/**
 * Same action list, rendered for the right-click context menu.
 */
export function renderContextMenuItems(item: ServiceActionItem, cb: ServiceActionsCallbacks) {
  const e = buildEntries(item, cb);
  return (
    <>
      {e.isRunning ? (
        <>
          <ContextMenuItem intent="danger" onAction={e.stop}>Stop</ContextMenuItem>
          <ContextMenuItem intent="danger" onAction={e.stopDisable}>Stop &amp; Disable</ContextMenuItem>
          <ContextMenuItem intent="danger" onAction={e.restart}>Restart</ContextMenuItem>
        </>
      ) : (
        <>
          <ContextMenuItem onAction={e.start}>Start</ContextMenuItem>
          {!item.daemon_enabled && (
            <ContextMenuItem onAction={e.enable}>Enable (auto-start)</ContextMenuItem>
          )}
        </>
      )}
      <ContextMenuItem onAction={e.showLogs}>Logs</ContextMenuItem>
      <ContextMenuItem onAction={e.configure}>Configure</ContextMenuItem>
      {e.isDocker && <ContextMenuItem onAction={e.pull}>Pull</ContextMenuItem>}
    </>
  );
}
