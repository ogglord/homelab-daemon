/**
 * Byte / IO-rate formatting helpers used across the storage page and its
 * sub-components. Extracted so each component file doesn't redefine them.
 */
export function fmtBytes(bytes: number): string {
  const sizes = ["B", "KB", "MB", "GB", "TB", "PB"];
  if (bytes === 0) return "0 B";
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${sizes[i]}`;
}

export function fmtSpeed(bytesPerSec: number): string {
  if (bytesPerSec === 0) return "0 B/s";
  const sizes = ["B/s", "KB/s", "MB/s", "GB/s"];
  const i = Math.floor(Math.log(bytesPerSec) / Math.log(1024));
  return `${(bytesPerSec / Math.pow(1024, i)).toFixed(1)} ${sizes[i]}`;
}

export interface DiskIORates {
  readBps: number;
  writeBps: number;
  readIops: number;
  writeIops: number;
}
