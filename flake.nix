{
  description = "homelab-daemon — service orchestrator + homelab CLI + web frontend";

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

        vendorHash = "sha256-YbPyurCsKsxSDgMe8jVWrKuOcyLmjuMa6Y8JJc0sKzs=";

        daemon = pkgs.buildGoModule {
          pname = "homelab-daemon";
          version = "0.1.0";
          src = ./.;
          inherit vendorHash;
          postPatch = ''
            rm -rf vendor
          '';
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

        cli = pkgs.buildGoModule {
          pname = "homelab";
          version = "0.1.0";
          src = ./.;
          inherit vendorHash;
          postPatch = ''
            rm -rf vendor
          '';
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

        frontend = pkgs.buildNpmPackage {
          name = "homelab-frontend";
          src = ./frontend;
          npmDepsHash = "sha256-GG7HFExwJ9M2rYZJE1Nk7yFF7qDMMBJNv9aAiLnklEI=";
          dontNpmBuild = true;
          npmFlags = [ "--loglevel=error" ];
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
            go gopls golangci-lint pkg-config libvirt nodejs tygo
          ];
          shellHook = ''
            echo "homelab-daemon dev shell (go $(go version | cut -d' ' -f3))"
            echo "node $(node --version) / npm $(npm --version)"
          '';
        };
      }
    )
    // {
      nixosModules.default = { pkgs, lib, ... }: {
        nixpkgs.overlays = [ (final: prev: {
          homelab-daemon = self.packages.${pkgs.system}.default;
          homelab-daemon-cli = self.packages.${pkgs.system}.cli;
          homelab-frontend = self.packages.${pkgs.system}.frontend;
        }) ];
        imports = [ ./module.nix ];
      };
    };
}
