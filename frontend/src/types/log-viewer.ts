/**
 * Type definitions for the log viewer feature.
 * Matches the JSON schema emitted by modules/log-viewer.nix.
 *
 * Uniform data model (enforced by Vector transforms):
 *   All logs have: _time, level (debug|info|warn|error), module, kind, _msg
 */

export interface LogViewerConfig {
  defaultMode: "query";
  defaultWindow: string;
  maxLines: number;
  services: LogViewerServiceConfig[];
}

export interface LogViewerServiceConfig {
  id: string;
  label: string;
  selector: string;           // LogsQL selector, e.g. {unit="homelab-daemon.service"}
  fields: LogFieldConfig[];   // filterable fields for this service
  format: "json" | "text" | "auto";
}

export interface LogFieldConfig {
  name: string;
  label: string;
  type: "enum" | "text" | "number";
  values?: string[];
}

export interface LogLine {
  _time: string;
  level?: string;
  module?: string;
  kind?: string;
  _msg?: string;
  unit?: string;
  [key: string]: unknown;
}
