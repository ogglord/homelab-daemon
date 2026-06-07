/**
 * Static navigation configuration for the dashboard sidebar.
 * Previously configured via HOMELAB_NAV_JSON / services.homelab-dash.nav
 * in configuration.nix. Now hardcoded in the frontend since the Go binary
 * that consumed that env var has been removed.
 *
 * Change this file to rearrange or add nav items; a rebuild is required.
 */
import type { NavItem } from "@/types";

export const navItems: NavItem[] = [
  {
    path: "/",
    label: "Overview",
  },
  {
    path: "/services",
    label: "Services",
  },
  {
    path: "/vms",
    label: "VMs",
  },
  {
    path: "/logs",
    label: "Logs",
  },
  {
    path: "",
    label: "Manage",
    children: [
      { path: "/storage", label: "Storage Pool" },
      { path: "/diagnostics", label: "Core Services" },
      { path: "/secrets", label: "Credentials" },
      { path: "https://files.cignl.cc", label: "Files" },
      { path: "/backups", label: "Backups" },
      { path: "https://filestash.cignl.cc", label: "Remote Files" },
      { path: "https://auth.cignl.cc", label: "Auth" },
    ],
  },
  {
    path: "",
    label: "Bookmarks",
    children: [
      { path: "https://homepage.cignl.cc", label: "Homepage" },
      { path: "https://immich.cignl.cc", label: "Immich" },
      { path: "https://plex.cignl.cc", label: "Plex" },
      { path: "https://jellyfin.cignl.cc", label: "Jellyfin" },
      { path: "https://sonarr.cignl.cc", label: "Sonarr" },
      { path: "https://radarr.cignl.cc", label: "Radarr" },
      { path: "https://prowlarr.cignl.cc", label: "Prowlarr" },
      { path: "https://code.cignl.cc", label: "Code Server" },
    ],
  },
];
