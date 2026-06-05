import { CpuWidget } from './system/CpuWidget';
import { MemoryWidget } from './system/MemoryWidget';
import { SystemInfoWidget } from './system/SystemInfoWidget';
import { StorageWidget } from './storage/StorageWidget';
import { ServicesWidget } from './services/ServicesWidget';
import { TemperatureWidget } from './system/TemperatureWidget';
import { UptimeWidget } from './system/UptimeWidget';
import { GpuEnginesWidget } from './system/GpuEnginesWidget';
import { GpuPowerWidget } from './system/GpuPowerWidget';
import { DisksWidget } from './system/DisksWidget';
import { NetworkWidget } from './system/NetworkWidget';
import { TopCpuWidget } from './system/TopCpuWidget';
import { TopMemoryWidget } from './system/TopMemoryWidget';
import { QbittorrentWidget } from './services/QbittorrentWidget';

export interface WidgetDefinition {
  id: string;
  name: string;
  component: React.ComponentType;
  defaultGridSpan?: number; // E.g. 1 for standard, 2 for wide
}

export interface WidgetLayout {
  id: string;
  span: 1 | 2 | 4;
}

export const WIDGETS: WidgetDefinition[] = [
  {
    id: 'cpu',
    name: 'CPU Usage',
    component: CpuWidget,
    defaultGridSpan: 2,
  },
  {
    id: 'memory',
    name: 'Memory Usage',
    component: MemoryWidget,
    defaultGridSpan: 2,
  },
  {
    id: 'system-info',
    name: 'System Info',
    component: SystemInfoWidget,
    defaultGridSpan: 1,
  },
  {
    id: 'temperature',
    name: 'Temperatures',
    component: TemperatureWidget,
    defaultGridSpan: 1,
  },
  {
    id: 'uptime',
    name: 'Uptime & Packages',
    component: UptimeWidget,
    defaultGridSpan: 1,
  },
  {
    id: 'gpu-engines',
    name: 'iGPU Engines',
    component: GpuEnginesWidget,
    defaultGridSpan: 1,
  },
  {
    id: 'gpu-power',
    name: 'iGPU Power/Freq',
    component: GpuPowerWidget,
    defaultGridSpan: 1,
  },
  {
    id: 'disks',
    name: 'System Disks',
    component: DisksWidget,
    defaultGridSpan: 1,
  },
  {
    id: 'network',
    name: 'Network Throughput',
    component: NetworkWidget,
    defaultGridSpan: 1,
  },
  {
    id: 'top-cpu',
    name: 'Top CPU Processes',
    component: TopCpuWidget,
    defaultGridSpan: 2,
  },
  {
    id: 'top-memory',
    name: 'Top Memory Processes',
    component: TopMemoryWidget,
    defaultGridSpan: 2,
  },
  {
    id: 'storage',
    name: 'Storage Pools',
    component: StorageWidget,
    defaultGridSpan: 1,
  },
  {
    id: 'services',
    name: 'Services Overview',
    component: ServicesWidget,
    defaultGridSpan: 1,
  },
  {
    id: 'qbittorrent',
    name: 'qBittorrent Active',
    component: QbittorrentWidget,
    defaultGridSpan: 2,
  },
];
