// Re-export generated API types. types.gen.ts is created during build
// by symlinking api-types/index.ts from the repo root. For local dev,
// run: ln -sf ../../api-types/index.ts src/types.gen.ts
//
// Frontend-only types (routing, navigation) are defined at the bottom.

import type {
  GpuStats, StatsSnapshot, DiskStat, Disk as StorageDisk,
  ProcessStat, HostInfo, PoolUsage, DiskIOStats,
  Pool as StoragePool, Subvolume as StorageSubvolume,
  StorageStatus, ServiceInfo, VMInfo,
} from "./types.gen";

export type { StorageDisk, StoragePool, StorageSubvolume, StorageStatus };
export type {
  GpuStats, DiskStat, ProcessStat, HostInfo, PoolUsage,
  DiskIOStats, ServiceInfo, VMInfo,
};
export type Stats = StatsSnapshot;
export type { StatsSnapshot, Pool, Disk, Subvolume, BackupStatus,
  Service, ServiceStatus, SuccessResponse, ErrorResponse,
  MountRequest, UnmountRequest, CommandResult,
  UpdateInfo, MetadataEntry, UpdatesStatus, VersionResponse,
  SecretEntry, PatchServiceRequest, PatchServiceResponse,
  PortMapping, VPNStatus, QbitStatus, QbitTorrent,
} from "./types.gen";

// ── Frontend-only types ────────────────────────────────────────────────

// ── Frontend-only types ────────────────────────────────────────────────

export interface IframeEntry {
  name: string;
  url: string;
  parent?: string;
}
export interface NavItem {
  path?: string;
  label: string;
  target?: string | null;
  is_active?: boolean;
  children?: NavItem[];
}
export interface NavGroup {
  parent: string;
  entries: IframeEntry[];
}
export interface PageData {
  hostname: string;
  nav: NavItem[];
  iframe_nav?: NavGroup[];
  active: string;
}
export interface APIResponse<T = unknown> {
  success: boolean;
  message?: string;
  error?: string;
  logs?: string;
  data?: T;
}
export type { DiskIORates } from "./pages/StoragePage/format";
