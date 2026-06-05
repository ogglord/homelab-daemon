# Rebuild workflow

## Quick reference

### I changed Go code
```bash
nix develop
go build ./cmd/daemon/
# or: go build ./cmd/cli/
```

### I changed API types (pkg/api/*.go)
```bash
nix develop --command bash -c '
  GOBIN=$(pwd) go install github.com/gzuidhof/tygo@latest
  cd pkg/api && ../tygo generate --config tygo.yaml
'
cp api-types/index.ts frontend/src/types.gen.ts
```

### I changed the frontend
```bash
nix develop
cd frontend && npm run dev   # hot-reload
# or: cd frontend && npm run build
```

### Deploy
```bash
git add -A && git commit -m "..." && git push
# Then in the nixos repo:
# nix flake update homelab-daemon
# nh os switch .
```

## tygo

The Go structs in `pkg/api/*.go` are the single source of truth.
Regenerate TypeScript types after any struct change:

```bash
nix develop --command bash -c '
  GOBIN=$(pwd) go install github.com/gzuidhof/tygo@latest
  cd pkg/api && ../tygo generate --config tygo.yaml
'
cp api-types/index.ts frontend/src/types.gen.ts
```

Never edit `frontend/src/types.gen.ts` or `api-types/index.ts`
manually.

## Common issues

| Issue | Fix |
|-------|-----|
| `unknown field X in struct literal` | `go mod tidy && go mod vendor` — vendor/ is stale |
| `cannot find module './types.gen'` | `cp api-types/index.ts frontend/src/types.gen.ts` |
| Import cycle in Go | Check `go.mod` `replace` directives |
