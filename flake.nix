{
  description = "homelab-daemon — service orchestrator + homelab CLI";

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
      in
      {
        packages = rec {
          homelab-daemon = pkgs.buildGoModule {
            pname = "homelab-daemon";
            version = "0.1.0";
            src = ./.;
            vendorHash = null;
            nativeBuildInputs = [ pkgs.pkg-config ];
            buildInputs = [ pkgs.libvirt ];
            subPackages = [
              "cmd/daemon"
              "cmd/homelab"
            ];
            postInstall = ''
              mv $out/bin/daemon $out/bin/homelab-daemon
            '';
            meta = {
              description = "Homelab daemon: service orchestration + CLI";
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
          ];
          shellHook = ''
            echo "homelab-daemon dev shell (go $(go version | cut -d' ' -f3))"
          '';
        };
      }
    );
}
