module github.com/ogglord/homelab-daemon

go 1.26.2

require (
	github.com/ogglord/homelab-api v0.0.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/shirou/gopsutil/v3 v3.24.5
	gopkg.in/yaml.v3 v3.0.1
	libvirt.org/go/libvirt v1.12003.0
)

require (
	github.com/ogglord/homelab-logging v0.0.0-00010101000000-000000000000
	github.com/prometheus/client_golang v1.23.2
	github.com/urfave/cli/v2 v2.27.7
	golang.org/x/sync v0.20.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/xrash/smetrics v0.0.0-20240521201337-686a1a2994c1 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	golang.org/x/sys v0.35.0 // indirect
	google.golang.org/protobuf v1.36.8 // indirect
)

// Shared wire-contract module. Local path; go.work resolves it for
// dev/CI, this replace keeps `go build` and Nix happy without the
// workspace.
replace github.com/ogglord/homelab-api => ./pkg/api

replace github.com/ogglord/homelab-logging => ./pkg/logging
