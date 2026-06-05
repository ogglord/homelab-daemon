{
  description = "homelab-daemon — service orchestrator + homelab CLI + web dashboard";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          config.allowUnfree = true;
        };

        # ── public packages ─────────────────────────────────────────────

        # The daemon HTTP server (privileged orchestrator).
        daemon = pkgs.buildGoModule {
          pname = "homelab-daemon";
          version = "0.1.0";
          src = ./.;
          vendorHash = null;
          nativeBuildInputs = [ pkgs.pkg-config ];
          buildInputs = [ pkgs.libvirt ];
          subPackages = [ "cmd/daemon" ];
          postInstall = ''
            mv $out/bin/daemon $out/bin/homelab-daemon
          '';
          meta = {
            description = "Homelab service orchestrator daemon";
            license = pkgs.lib.licenses.mit;
            mainProgram = "homelab-daemon";
          };
        };

        # The homelab CLI (talks to the daemon over its Unix socket).
        cli = pkgs.buildGoModule {
          pname = "homelab";
          version = "0.1.0";
          src = ./.;
          vendorHash = null;
          nativeBuildInputs = [ pkgs.pkg-config ];
          buildInputs = [ pkgs.libvirt ];
          subPackages = [ "cmd/cli" ];
          postInstall = ''
            mv $out/bin/cli $out/bin/homelab
          '';
          meta = {
            description = "Homelab CLI — manage services, secrets, diagnostics";
            license = pkgs.lib.licenses.mit;
            mainProgram = "homelab";
          };
        };

        # The web frontend (React/Vite, served by Caddy).
        frontend = pkgs.buildNpmPackage {
          name = "homelab-frontend";
          src = ./frontend;
          npmDepsHash = "sha256-KIGUZ3+9Kq1zDS8bjZMI7C7+sk5cV9nZunZuwGq+22E=";
          dontNpmBuild = true;
          npmFlags = [ "--loglevel=error" ];
          preBuild = ''
            # Ensure generated types from pkg/api are available to the build.
            # This symlink makes src/types.gen.ts point at the generated file.
            mkdir -p src
            ln -sf ../../api-types/index.ts src/types.gen.ts
          '';
          buildPhase = ''
            runHook preBuild
            npm run build
            runHook postBuild
          '';
          installPhase = ''
            runHook preInstall
            mkdir -p $out
            cp -r dist/* $out/
            runHook postInstall
          '';
        };

      in
      {
        packages = rec {
          inherit daemon cli frontend;
          # Combined: both daemon + CLI (for legacy compatibility).
          homelab-daemon = pkgs.symlinkJoin {
            name = "homelab-daemon";
            paths = [ daemon cli ];
            meta = {
              description = "Homelab daemon + CLI";
              license = pkgs.lib.licenses.mit;
              mainProgram = "homelab-daemon";
            };
          };
          default = homelab-daemon;
        };

        devShells.default = pkgs.mkShell {
          name = "homelab-daemon-dev";
          buildInputs = with pkgs; [
            go
            gopls
            golangci-lint
            pkg-config
            libvirt
            nodejs
          ];
          shellHook = ''
            echo "homelab-daemon dev shell (go $(go version | cut -d' ' -f3))"
            echo "node $(node --version) / npm $(npm --version)"
          '';
        };
      }
    )
    # ── NixOS module ────────────────────────────────────────────────────
    // {
      nixosModules.default = { pkgs, lib, ... }: {
        options.services.homelab-daemon = {
          enable = lib.mkEnableOption "homelab service orchestrator";
          dash = lib.mkOption {
            type = lib.types.bool;
            default = true;
            description = "Enable the web dashboard frontend.";
          };
        };

        config = lib.mkIf (lib.attrByPath [ "services" "homelab-daemon" "enable" ] false { }) {
          environment.systemPackages = [
            self.packages.${pkgs.system}.cli
          ];
        };
      };
    };
}
