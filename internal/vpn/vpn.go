package vpn

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	logging "github.com/ogglord/homelab-logging"

	"github.com/ogglord/homelab-daemon/internal/cmdrunner"
)

var log = logging.Logger("vpn")

// Config mirrors main.VPNConfig; main maps its parsed config into this.
type Config struct {
	Enabled                bool
	NetnsName              string
	Interface              string
	Address                string
	DNS                    string
	PeerPublicKey          string
	PeerEndpoint           string
	AllowedIPs             string
	PrivateKeyFile         string
	Provider               string
	Type                   string
	ServerCountry          string
	VethHostIP             string
	VethNetnsIP            string
	PortFile               string
	RefreshIntervalSeconds int
}

type Module struct {
	cfg       Config
	cache     *stateCache
	reconnect chan struct{}
	failures  int
	mu        sync.Mutex
}

func New(cfg Config) *Module {
	m := &Module{
		cfg:       cfg,
		cache:     newStateCache(),
		reconnect: make(chan struct{}, 1),
	}
	// Seed the snapshot so the widget shows provider/type even before the
	// first poll (and when disabled).
	m.cache.set(Status{
		Enabled:       cfg.Enabled,
		Provider:      cfg.Provider,
		Type:          cfg.Type,
		ServerCountry: cfg.ServerCountry,
	})
	return m
}

func (m *Module) Get() Status { return m.cache.get() }

func (m *Module) Reconnect() {
	select {
	case m.reconnect <- struct{}{}:
	default:
	}
}

func (m *Module) paramsFromConfig() Params {
	return Params{
		NetnsName:      m.cfg.NetnsName,
		Interface:      m.cfg.Interface,
		Address:        m.cfg.Address,
		DNS:            m.cfg.DNS,
		PeerPublicKey:  m.cfg.PeerPublicKey,
		PeerEndpoint:   m.cfg.PeerEndpoint,
		AllowedIPs:     m.cfg.AllowedIPs,
		PrivateKeyFile: m.cfg.PrivateKeyFile,
		VethHostIP:     m.cfg.VethHostIP,
		VethNetnsIP:    m.cfg.VethNetnsIP,
	}
}

// run executes a single argv slice via cmdrunner. The first element is the
// command, the rest are args.
func (m *Module) run(ctx context.Context, argv []string) (cmdrunner.Result, error) {
	return cmdrunner.New("vpn", argv[0], argv[1:]...).WithContext(ctx).Run()
}

// setup (re)establishes the netns + tunnel. "exists" failures on create
// steps are logged and ignored so setup is safely re-runnable.
func (m *Module) setup(ctx context.Context) {
	for _, argv := range BuildSetupCommands(m.paramsFromConfig()) {
		if _, err := m.run(ctx, argv); err != nil {
			log.Warn("vpn setup step failed (may be benign if already present)",
				"cmd", strings.Join(argv, " "), "error", err)
		}
	}
}

type portError struct{}

func (*portError) Error() string { return "natpmpc returned no port" }

var errNoPort = &portError{}

// refreshPort runs natpmpc for udp+tcp, publishes the port, returns it.
func (m *Module) refreshPort(ctx context.Context) (int, error) {
	ns := m.cfg.NetnsName
	gw := m.cfg.DNS // ProtonVPN NAT-PMP gateway == tunnel gateway (e.g. 10.2.0.1)
	var port int
	for _, proto := range []string{"udp", "tcp"} {
		res, err := m.run(ctx, []string{"ip", "netns", "exec", ns,
			"natpmpc", "-a", "1", "0", proto, "60", "-g", gw})
		if err != nil {
			return 0, err
		}
		if p, perr := ParseMappedPort(res.Stdout); perr == nil {
			port = p
		}
	}
	if port == 0 {
		return 0, errNoPort
	}
	if m.cfg.PortFile != "" {
		if err := PublishPort(m.cfg.PortFile, port); err != nil {
			log.Error("publish forwarded port failed", "error", err)
		}
	}
	return port, nil
}

// checkHandshake returns the latest-handshake epoch via `wg show`.
func (m *Module) checkHandshake(ctx context.Context) (int64, error) {
	res, err := m.run(ctx, []string{"ip", "netns", "exec", m.cfg.NetnsName,
		"wg", "show", m.cfg.Interface, "latest-handshakes"})
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(res.Stdout)
	if len(fields) < 2 {
		return 0, nil
	}
	ts, _ := strconv.ParseInt(fields[len(fields)-1], 10, 64)
	return ts, nil
}

func (m *Module) Start(ctx context.Context) {
	if !m.cfg.Enabled {
		log.Info("vpn module disabled; not starting")
		return
	}
	log.Info("starting vpn module", "netns", m.cfg.NetnsName, "provider", m.cfg.Provider)
	m.setup(ctx)

	interval := time.Duration(m.cfg.RefreshIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 45 * time.Second
	}
	portTicker := time.NewTicker(interval)
	defer portTicker.Stop()
	healthTicker := time.NewTicker(30 * time.Second)
	defer healthTicker.Stop()

	m.tick(ctx) // initial sample

	for {
		select {
		case <-ctx.Done():
			log.Info("stopping vpn module")
			return
		case <-m.reconnect:
			log.Info("vpn reconnect requested")
			m.setup(ctx)
			m.tick(ctx)
		case <-portTicker.C:
			m.tick(ctx)
		case <-healthTicker.C:
			ts, err := m.checkHandshake(ctx)
			stale := err != nil || (ts > 0 && time.Since(time.Unix(ts, 0)) > 3*time.Minute)
			if stale {
				m.mu.Lock()
				m.failures++
				m.mu.Unlock()
				log.Warn("vpn handshake stale; reconnecting", "last_handshake", ts, "error", err)
				m.setup(ctx)
			}
		}
	}
}

// tick refreshes the port + public IP and updates the cache.
func (m *Module) tick(ctx context.Context) {
	s := Status{
		Enabled:       true,
		Provider:      m.cfg.Provider,
		Type:          m.cfg.Type,
		ServerCountry: m.cfg.ServerCountry,
	}
	if port, err := m.refreshPort(ctx); err != nil {
		s.ErrMsg = err.Error()
	} else {
		s.ForwardedPort = port
		s.Connected = true
	}
	if ip, err := m.run(ctx, []string{"ip", "netns", "exec", m.cfg.NetnsName,
		"curl", "-s", "--max-time", "5", "https://ipinfo.io/ip"}); err == nil {
		s.PublicIP = strings.TrimSpace(ip.Stdout)
	}
	if ts, err := m.checkHandshake(ctx); err == nil && ts > 0 {
		s.LastHandshake = time.Unix(ts, 0).Format(time.RFC3339)
	}
	m.mu.Lock()
	s.FailureCount = m.failures
	m.mu.Unlock()
	m.cache.set(s)
}
