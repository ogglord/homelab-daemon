{
  config,
  pkgs,
  lib,
  ...
}:

# homelab-daemon — NixOS module.
#
# Provides the service orchestrator daemon, CLI, and web frontend.
# Invasive operations (service lifecycle management, firewall rules,
# pool protection) are gated behind separate `.enable` options.
#
# Usage:
#   inputs.homelab-daemon.nixosModules.default
#
# The module registers an overlay that makes these available in pkgs:
#   pkgs.homelab-daemon     — daemon + CLI (symlinkJoin)
#   pkgs.homelab-daemon-cli — CLI only
#   pkgs.homelab-frontend   — frontend static files

let
  cfg = config.services.homelab-daemon;
  pkg = pkgs.homelab-daemon; # combined daemon + CLI
  cliPkg = pkgs.homelab-daemon-cli;
  frontendPkg = pkgs.homelab-frontend;

  isPodman = key: lib.hasPrefix "podman-" key;
  toContainer = key: lib.removePrefix "podman-" key;
  toUnit = key: key + ".service";

  sortedServices = lib.sort (a: b: if a.order != b.order then a.order < b.order else a.key < b.key) (
    lib.mapAttrsToList (key: svcCfg: { inherit key; } // svcCfg) cfg.managedServices
  );

  yamlEntry =
    svc:
    "  - unit: ${toUnit svc.key}\n"
    + "    enabled: true\n"
    + "    order: ${toString svc.order}\n"
    + "    boot_delay: ${toString svc.bootDelay}\n"
    + "    restart: ${svc.restart}\n"
    + "    restart_delay: ${toString svc.restartDelay}\n"
    + lib.optionalString (svc.dependsOn != [ ]) (
      "    depends_on:\n" + lib.concatMapStrings (dep: "      - ${dep}\n") svc.dependsOn
    )
    + lib.optionalString (svc.requiresMounts != [ ]) (
      "    requires_mount:\n" + lib.concatMapStrings (mnt: "      - ${mnt}\n") svc.requiresMounts
    )
    + lib.optionalString (svc.icon != null) "    icon_url: ${lib.escapeShellArg svc.icon}\n"
    + lib.optionalString (svc.homepage != null) "    homepage_url: ${lib.escapeShellArg svc.homepage}\n"
    + "\n";

  yamlBackupEntry =
    key: backupCfg:
    "  - unit: ${toUnit key}\n"
    + "    enabled: true\n"
    + "    schedule: \"${backupCfg.schedule}\"\n"
    + lib.optionalString (backupCfg.dependsOn != [ ]) (
      "    depends_on:\n" + lib.concatMapStrings (dep: "      - ${dep}\n") backupCfg.dependsOn
    )
    + lib.optionalString (backupCfg.requiresMounts != [ ]) (
      "    requires_mount:\n" + lib.concatMapStrings (mnt: "      - ${mnt}\n") backupCfg.requiresMounts
    )
    + "\n";

  defaultServicesYaml = ''
    # homelab-daemon — service orchestration config
    # Generated from managedServices declarations. Edit freely.
    version: 1
    services:

  ''
  + lib.concatMapStrings yamlEntry sortedServices
  + lib.optionalString (cfg.managedBackups != { }) (
    "\nbackups:\n\n" + lib.concatStrings (lib.mapAttrsToList yamlBackupEntry cfg.managedBackups)
  )
  + lib.optionalString (cfg.notify.smtp.host != "") (
    "\nnotify:\n"
    + "  smtp:\n"
    + "    host: ${lib.escapeNixString cfg.notify.smtp.host}\n"
    + "    port: ${toString cfg.notify.smtp.port}\n"
    + "    username: ${lib.escapeNixString cfg.notify.smtp.username}\n"
    + "    password: ${lib.escapeNixString cfg.notify.smtp.password}\n"
    + "  from: ${lib.escapeNixString cfg.notify.from}\n"
    + "  to: ${lib.escapeNixString cfg.notify.to}\n"
  )
  + lib.optionalString cfg.vpn.enable (
    "\nvpn:\n"
    + "  enabled: true\n"
    + "  netns_name: ${builtins.toJSON cfg.vpn.netnsName}\n"
    + "  interface: ${builtins.toJSON cfg.vpn.interface}\n"
    + "  address: ${builtins.toJSON cfg.vpn.address}\n"
    + "  dns: ${builtins.toJSON cfg.vpn.dns}\n"
    + "  peer_public_key: ${builtins.toJSON cfg.vpn.peerPublicKey}\n"
    + "  peer_endpoint: ${builtins.toJSON cfg.vpn.peerEndpoint}\n"
    + "  allowed_ips: ${builtins.toJSON cfg.vpn.allowedIPs}\n"
    + "  private_key_file: ${builtins.toJSON cfg.vpn.privateKeyFile}\n"
    + "  provider: ${builtins.toJSON cfg.vpn.provider}\n"
    + "  type: ${builtins.toJSON cfg.vpn.type}\n"
    + "  server_country: ${builtins.toJSON cfg.vpn.serverCountry}\n"
    + "  veth_host_ip: ${builtins.toJSON cfg.vpn.vethHostIP}\n"
    + "  veth_netns_ip: ${builtins.toJSON cfg.vpn.vethNetnsIP}\n"
    + "  port_file: ${builtins.toJSON cfg.vpn.portFile}\n"
    + "  refresh_interval_seconds: ${toString cfg.vpn.refreshIntervalSeconds}\n"
  );

in
{
  options.services.homelab-daemon = {
    enable = lib.mkEnableOption "homelab service orchestrator" // {
      default = true;
    };

    enableManagedServices = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = ''
        Whether the daemon controls systemd unit lifecycle. When enabled,
        strips wantedBy from native services and sets autoStart=false on
        podman containers — the daemon decides when to start/stop/restart.
        Disable if you want systemd to manage units directly.
      '';
    };

    enableFirewall = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = ''
        Open firewall ports (cluster HTTPS 53443, DNS TCP). Off by default;
        the consuming flake typically manages its own firewall rules.
      '';
    };

    enablePoolProtection = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = ''
        Set chattr +i on /pool at boot. Prevents accidental writes to the
        bare directory when bcachefs isn't mounted. The daemon removes the
        flag before auto-mounting.
      '';
    };

    configFile = lib.mkOption {
      type = lib.types.str;
      default = "/cache/appdata/homelab/services.yaml";
    };

    flakePath = lib.mkOption {
      type = lib.types.str;
      default = "";
      description = "Absolute path to the NixOS flake root. Used by the daemon to locate secrets.yaml and .sops.yaml for sops operations. Set this in your host configuration, e.g. /home/ogge/repos/nixos.";
    };

    stateDir = lib.mkOption {
      type = lib.types.str;
      default = "/var/lib/homelab-daemon";
    };

    notify = lib.mkOption {
      type = lib.types.submodule {
        options = {
          smtp = {
            host = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = "SMTP server hostname for email notifications.";
            };
            port = lib.mkOption {
              type = lib.types.port;
              default = 587;
              description = "SMTP server port.";
            };
            username = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = "SMTP username.";
            };
            password = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = "SMTP password or app token.";
            };
          };
          from = lib.mkOption {
            type = lib.types.str;
            default = "homelab-daemon@${config.networking.hostName}";
            description = "From address for notification emails.";
          };
          to = lib.mkOption {
            type = lib.types.str;
            default = "";
            description = "Recipient address for notification emails.";
          };
        };
      };
      default = { };
      description = ''
        SMTP notification settings. When configured, the daemon sends email
        alerts for failed backup jobs, service failures, and daemon crashes.
      '';
    };

    vpn = lib.mkOption {
      description = "Daemon-owned WireGuard netns (generic VPN infra; no consumer knowledge).";
      default = { };
      type = lib.types.submodule {
        options = {
          enable = lib.mkEnableOption "daemon-managed WireGuard VPN netns";
          netnsName = lib.mkOption {
            type = lib.types.str;
            default = "vpn";
          };
          interface = lib.mkOption {
            type = lib.types.str;
            default = "wg0";
          };
          address = lib.mkOption {
            type = lib.types.str;
            default = "";
          };
          dns = lib.mkOption {
            type = lib.types.str;
            default = "10.2.0.1";
          };
          peerPublicKey = lib.mkOption {
            type = lib.types.str;
            default = "";
          };
          peerEndpoint = lib.mkOption {
            type = lib.types.str;
            default = "";
          };
          allowedIPs = lib.mkOption {
            type = lib.types.str;
            default = "0.0.0.0/0";
          };
          privateKeyFile = lib.mkOption {
            type = lib.types.str;
            default = "";
          };
          provider = lib.mkOption {
            type = lib.types.str;
            default = "protonvpn";
          };
          type = lib.mkOption {
            type = lib.types.str;
            default = "wireguard";
          };
          serverCountry = lib.mkOption {
            type = lib.types.str;
            default = "Switzerland";
          };
          vethHostIP = lib.mkOption {
            type = lib.types.str;
            default = "10.200.0.1/30";
          };
          vethNetnsIP = lib.mkOption {
            type = lib.types.str;
            default = "10.200.0.2/30";
          };
          portFile = lib.mkOption {
            type = lib.types.str;
            default = "/run/homelab-daemon/vpn/forwarded-port";
          };
          refreshIntervalSeconds = lib.mkOption {
            type = lib.types.int;
            default = 45;
          };
        };
      };
    };

    managedServices = lib.mkOption {
      type = lib.types.attrsOf (
        lib.types.submodule {
          options = {
            order = lib.mkOption {
              type = lib.types.int;
              default = 10;
            };
            bootDelay = lib.mkOption {
              type = lib.types.int;
              default = 0;
            };
            restart = lib.mkOption {
              type = lib.types.enum [
                "no"
                "on-failure"
                "unless-stopped"
                "always"
              ];
              default = "unless-stopped";
            };
            restartDelay = lib.mkOption {
              type = lib.types.int;
              default = 10;
            };
            dependsOn = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
            };
            requiresMounts = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
            };
            icon = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = "URL to an icon SVG for this service (e.g. https://selfh.st/icons/immich.svg).";
            };
            homepage = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = "Homepage URL for this service (e.g. https://immich.cignl.cc).";
            };
          };
        }
      );
      default = { };
      description = "Services managed by the daemon.";
    };

    managedBackups = lib.mkOption {
      type = lib.types.attrsOf (
        lib.types.submodule {
          options = {
            schedule = lib.mkOption {
              type = lib.types.str;
              default = "0 4 * * *";
            };
            dependsOn = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
            };
            requiresMounts = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
            };
          };
        }
      );
      default = { };
      description = "Backup jobs managed by the daemon.";
    };

    enableDoctorOnActivation = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = ''
        Run `homelab doctor` after each NixOS activation (nh os switch).
        Results are written to the journal (tag: homelab-doctor).
        Failures emit an SMTP notification but never block the switch.
      '';
    };
  };

  config = lib.mkIf cfg.enable {
    # ── Packages ───────────────────────────────────────────────────────
    environment.systemPackages = [ cliPkg ];

    # ── System user ────────────────────────────────────────────────────
    users.users.homelab-daemon = {
      isSystemUser = true;
      group = "homelab-daemon";
      description = "Homelab service daemon";
      shell = pkgs.bashInteractive;
    };
    users.groups.homelab-daemon = { };

    # Allow caddy to connect to the daemon unix socket.
    users.users.caddy.extraGroups = lib.mkIf (config.services.caddy.enable or false) [
      "homelab-daemon"
    ];

    # ── Pool protection ────────────────────────────────────────────────
    system.activationScripts.immutablePool = lib.mkIf cfg.enablePoolProtection ''
      if ! mountpoint -q /pool 2>/dev/null; then
        ${pkgs.e2fsprogs}/bin/chattr +i /pool
      fi
    '';

    system.activationScripts.homelabDoctor = lib.mkIf cfg.enableDoctorOnActivation {
      deps = [ "specialfs" ];
      text = ''
        ${pkgs.homelab-daemon}/bin/homelab doctor --json --check disk,systemd-units 2>&1 \
          | ${pkgs.systemd}/bin/systemd-cat -t homelab-doctor -p info || true
      '';
    };

    # ── Validation ─────────────────────────────────────────────────────
    assertions = lib.mkIf cfg.enableManagedServices (
      (lib.mapAttrsToList (key: _: {
        assertion = !(lib.hasSuffix ".service" key);
        message = "homelab-daemon: managedServices key \"${key}\" must not include .service suffix.";
      }) cfg.managedServices)

      ++ (lib.mapAttrsToList (key: _: {
        assertion = (config.systemd.services.${key}.serviceConfig or { }) != { };
        message = "homelab-daemon: managedServices.\"${key}\" does not match any real systemd service.";
      }) (lib.filterAttrs (key: _: !isPodman key) cfg.managedServices))

      ++ (lib.mapAttrsToList (key: _: {
        assertion = (config.virtualisation.oci-containers.containers.${toContainer key}.image or "") != "";
        message = "homelab-daemon: managedServices.\"${key}\" (container \"${toContainer key}\") has no image defined.";
      }) (lib.filterAttrs (key: _: isPodman key) cfg.managedServices))
    );

    # ── Config directory + default services.yaml ───────────────────────
    systemd.tmpfiles.rules = [ "d /cache/appdata/homelab 0755 homelab-daemon homelab-daemon -" ];

    system.activationScripts.homelab-daemon-config = {
      text = ''
                mkdir -p "$(dirname ${lib.escapeShellArg cfg.configFile})"
                chown homelab-daemon:homelab-daemon "$(dirname ${lib.escapeShellArg cfg.configFile})"
                TMP_DEFAULT=$(mktemp)
                cat > "$TMP_DEFAULT" <<'YAML'
        ${defaultServicesYaml}
        YAML
                ${pkg}/bin/homelab-daemon merge-config \
                  --config ${lib.escapeShellArg cfg.configFile} \
                  --defaults "$TMP_DEFAULT"
                rm -f "$TMP_DEFAULT"
                chown homelab-daemon:homelab-daemon ${lib.escapeShellArg cfg.configFile}
      '';
    };

    # ── Managed-units registry ─────────────────────────────────────────
    environment.etc."homelab-daemon/managed-units" = lib.mkIf cfg.enableManagedServices {
      text = lib.concatMapStrings (key: toUnit key + "\n") (
        lib.sort lib.lessThan (lib.attrNames cfg.managedServices ++ lib.attrNames cfg.managedBackups)
      );
      mode = "0444";
    };

    # ── Systemd: lifecycle overrides + daemon service ──────────────────
    systemd.services =
      let
        managedOverrides = lib.mapAttrs' (
          key: _:
          lib.nameValuePair key {
            wantedBy = lib.mkForce [ ];
            restartIfChanged = false;
            stopIfChanged = false;
          }
        ) cfg.managedServices;

        backupOverrides = lib.mapAttrs' (
          key: _:
          lib.nameValuePair key {
            wantedBy = lib.mkForce [ ];
            restartIfChanged = false;
            stopIfChanged = false;
          }
        ) cfg.managedBackups;
      in
      lib.mkMerge ([
        (lib.mkIf cfg.enableManagedServices managedOverrides)
        (lib.mkIf cfg.enableManagedServices backupOverrides)
        (lib.mkIf cfg.enableDoctorOnActivation {
          homelab-doctor-report = {
            description = "Homelab post-activation doctor report";
            wantedBy = [ "multi-user.target" ];
            requires = [ "homelab-daemon.service" ];
            after = [
              "homelab-daemon.service"
              "network-online.target"
            ];
            serviceConfig = {
              Type = "oneshot";
              RemainAfterExit = true;
              # Wait up to 30s for the daemon socket to appear before running checks.
              ExecStartPre = "/bin/sh -c 'for i in $(seq 30); do [ -S /run/homelab-daemon/daemon.sock ] && exit 0; sleep 1; done; exit 1'";
              ExecStart = "${pkgs.homelab-daemon}/bin/homelab doctor --json --fail-on-error";
              ExecStopPost = "/bin/sh -c '${pkgs.homelab-daemon}/bin/homelab doctor --json | ${pkgs.homelab-daemon}/bin/homelab doctor notify || true'";
              StandardOutput = "journal";
              StandardError = "journal";
              SyslogIdentifier = "homelab-doctor";
            };
          };
        })
        {
          homelab-daemon = {
            description = "Homelab service orchestrator";
            wantedBy = [ "multi-user.target" ];
            after = [ "network.target" ];
            path = with pkgs; [
              bash
              util-linux
              bcachefs-tools
              systemd
              podman
              skopeo
              git
              openssh
              smartmontools
              sops
              age
              nix
              nh
              pi-coding-agent
              sandbox-runtime
              # VPN subsystem: WireGuard netns bring-up + NAT-PMP port forwarding.
              wireguard-tools
              iproute2
              libnatpmp
              curl
            ];
            environment = lib.mkIf (cfg.flakePath != "") {
              NH_FLAKE = cfg.flakePath;
              SOPS_CONFIG = "${cfg.flakePath}/.sops.yaml";
            };
            serviceConfig = {
              ExecStart =
                "${pkg}/bin/homelab-daemon" + " --config ${cfg.configFile}" + " --state-dir ${cfg.stateDir}";
              Restart = "on-failure";
              RestartSec = "5s";
              RuntimeDirectory = "homelab-daemon";
              RuntimeDirectoryMode = "0750";
              UMask = "0027";
              StateDirectory = "homelab-daemon";
              StateDirectoryMode = "0750";
              User = "root";
              Group = "homelab-daemon";
              LockPersonality = true;
              RestrictAddressFamilies = [
                "AF_UNIX"
                "AF_INET"
                "AF_NETLINK"
              ];
            };
          };
        }
      ]);

    # ── Podman autoStart suppression ───────────────────────────────────
    virtualisation.oci-containers.containers = lib.mkIf cfg.enableManagedServices (
      lib.mapAttrs' (key: _: lib.nameValuePair (toContainer key) { autoStart = lib.mkForce false; }) (
        lib.filterAttrs (key: _: isPodman key) cfg.managedServices
      )
    );

    # ── Firewall ───────────────────────────────────────────────────────
    networking.firewall.extraCommands = lib.mkIf cfg.enableFirewall ''
      iptables -A nixos-fw -p tcp -s 192.168.0.10 --dport 53443 -j nixos-fw-accept
      iptables -A nixos-fw -p tcp -s 192.168.0.0/24 --dport 53 -j nixos-fw-accept
    '';
  };
}
