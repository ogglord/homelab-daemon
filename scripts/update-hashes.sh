#!/usr/bin/env bash
# scripts/update-hashes.sh
# Usage: ./scripts/update-hashes.sh [daemon|frontend|all]
set -euo pipefail

TARGET=${1:-all}
FLAKE="$(git rev-parse --show-toplevel)/flake.nix"

get_hash() {
  local pkg=$1
  echo "==> Fetching new hash for ${pkg}..."
  # Run nix build and parse the error output for the 'got:' hash.
  # nix build exits 1 on hash mismatch — we need to survive that so
  # grep can extract the corrected hash from stderr.
  ( nix build ".#${pkg}" --no-link 2>&1 || true ) | grep "got:" | awk '{print $2}' | tail -1
}

patch_hash() {
  local field=$1 hash=$2
  # Replace the matching field value in flake.nix
  sed -i "s|${field} = \"sha256-[^\"]*\"|${field} = \"${hash}\"|" "$FLAKE"
}

# Ensure changes to flake.nix are staged so Nix sees them
stage_flake() {
  git add "$FLAKE"
}

if [[ "$TARGET" == "daemon" || "$TARGET" == "all" ]]; then
  echo "==> Refreshing daemon vendorHash..."
  patch_hash vendorHash "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
  stage_flake
  HASH=$(get_hash daemon)
  if [[ -n "$HASH" ]]; then
    patch_hash vendorHash "$HASH"
    stage_flake
    echo "Successfully updated vendorHash to: $HASH"
  else
    echo "Failed to get new vendorHash. Is the build succeeding without hash failure?" >&2
  fi
fi

if [[ "$TARGET" == "frontend" || "$TARGET" == "all" ]]; then
  echo "==> Refreshing frontend npmDepsHash..."
  patch_hash npmDepsHash "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
  stage_flake
  HASH=$(get_hash frontend)
  if [[ -n "$HASH" ]]; then
    patch_hash npmDepsHash "$HASH"
    stage_flake
    echo "Successfully updated npmDepsHash to: $HASH"
  else
    echo "Failed to get new npmDepsHash. Is the build succeeding without hash failure?" >&2
  fi
fi

echo "Done. Verifying build..."
if [[ "$TARGET" == "all" ]]; then
  nix build .#daemon .#frontend --no-link
else
  nix build ".#$TARGET" --no-link
fi
echo "Build verification successful!"
