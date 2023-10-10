package dialog

import (
	"log/slog"
	"net"
	"net/netip"
	"time"

	"github.com/safermobility/sipmanager/sip"
)

type Manager struct {
	logger *slog.Logger

	// looseSignaling bool // Permit SIP messages from servers other than the next hop
	maxResends       int            // How many times to try resending non-ACK'ed packets
	rawTrace         bool           // Whether to print the raw messages in the log
	resendInterval   time.Duration  // How long to wait before trying to resend non-ACK'ed messages
	timestampTagging bool           // Add timestamps to Via headers for debugging
	userAgent        string         // The `User-Agent` header value
	listenAddress    string         // defaults to empty string = "all addresses on a random port"
	publicAddrPort   netip.AddrPort // If behind 1-to-1 NAT, this IP will be considered our local address
	proxyAddress     *net.UDPAddr   // If set, send all messages to the proxy instead of directly to the destination
	allowReinvite    bool           // Whether to allow RFC 3725/4117 re-INVITE or not

	sock    *net.UDPConn
	contact *sip.Addr // The local (or public IP, if set) Contact for this server
	via     *sip.Via  // The local (or public IP, if set) Via for this server

	dialogs map[sip.CallID]*dialogState
}

const (
	defaultMaxResends       = 2
	defaultRawTrace         = false
	defaultResendInterval   = time.Second
	defaultTimestampTagging = false
	defaultUserAgent        = "sipmanager/1.0"
)

func NewManager(opts ...ManagerOption) (*Manager, error) {
	m := &Manager{
		maxResends:       defaultMaxResends,
		rawTrace:         defaultRawTrace,
		resendInterval:   defaultResendInterval,
		timestampTagging: defaultTimestampTagging,
		userAgent:        defaultUserAgent,

		dialogs: make(map[sip.CallID]*dialogState),
	}

	for _, opt := range opts {
		if err := opt(m); err != nil {
			// TODO: return only one, or a multi-error?
			// TODO: return the partially-configured new object?
			return nil, err
		}
	}

	sock, err := net.ListenPacket("udp", m.listenAddress)
	if err != nil {
		return nil, err
	}
	m.sock = sock.(*net.UDPConn)

	m.contact = &sip.Addr{
		Uri: &sip.URI{
			Host: m.PublicAddress().String(),
			Port: m.PublicPort(),
			Param: &sip.URIParam{
				Name:  "transport",
				Value: "udp",
			},
		},
	}
	m.via = &sip.Via{
		Host: m.PublicAddress().String(),
		Port: m.PublicPort(),
	}

	go m.ReceiveMessages()

	return m, nil
}

// LocalPort returns the local port number that is being used to receive SIP traffic
func (m *Manager) LocalPort() uint16 {
	return uint16(m.sock.LocalAddr().(*net.UDPAddr).Port)
}

// PublicAddress returns the configured public IP address, if configured,
// or the local IP address that is being used to receive SIP traffic
func (m *Manager) PublicAddress() netip.Addr {
	if m.publicAddrPort.IsValid() {
		return m.publicAddrPort.Addr()
	}

	return m.sock.LocalAddr().(*net.UDPAddr).AddrPort().Addr()
}

// PublicPort returns the configured port, if configured,
// or the local port that is being used to receive SIP traffic
func (m *Manager) PublicPort() uint16 {
	if m.publicAddrPort.IsValid() {
		return m.publicAddrPort.Port()
	}

	return uint16(m.sock.LocalAddr().(*net.UDPAddr).Port)
}
