.PHONY: types vendor preflight deploy

MSG ?= minor changes

# Regenerate TypeScript types from pkg/api/*.go
types:
	nix develop --command bash -c 'cd pkg/api && tygo generate --config tygo.yaml'
	cp api-types/index.ts frontend/src/types.gen.ts
	@echo "Regenerated frontend/src/types.gen.ts"

# Sync vendor locally (only needed for editor LSP; Nix builds re-vendor via vendorHash)
vendor:
	nix develop --command bash -c 'go mod tidy && go mod vendor'

# Run code vetting and test builds
preflight: types
	nix develop --command bash -c 'go vet ./...'
	nix develop --command bash -c 'go build ./cmd/daemon/ ./cmd/cli/'
	@echo "All pre-flight checks passed."

# Commit, push, and remind to run nix flake update on the NixOS system
deploy: preflight
	git add -A
	git commit -m "$(MSG)" || true
	git push
	@echo ""
	@echo ">>> Now in the nixos repo run:"
	@echo "    nix flake update homelab-daemon && nh os switch ."
