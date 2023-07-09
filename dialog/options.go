package dialog

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"time"

	"golang.org/x/exp/slog"
)

type ManagerOption func(*Manager) error

var (
	ErrAddrPortAlreadySet   = errors.New("socket listen address/port can only be set once")
	ErrProxyAddressNotValid = errors.New("proxy address is not valid")
)

// Select the local listening address and port
func WithListenAddrPort(a netip.AddrPort) ManagerOption {
	return func(m *Manager) error {
		if m.listenAddress != "" {
			return ErrAddrPortAlreadySet
		}

		m.listenAddress = a.String()

		return nil
	}
}

// Select the local listening port (on all addresses)
func WithListenPort(port uint16) ManagerOption {
	return func(m *Manager) error {
		if m.listenAddress != "" {
			return ErrAddrPortAlreadySet
		}

		m.listenAddress = fmt.Sprintf(":%d", port)

		return nil
	}
}

// Select the local listening address:port
func WithListenString(address string) ManagerOption {
	return func(m *Manager) error {
		if m.listenAddress != "" {
			return ErrAddrPortAlreadySet
		}

		m.listenAddress = address

		return nil
	}
}

// func WithLooseSignaling(value bool) ManagerOption {
// 	return func(m *Manager) error {
// 		m.looseSignaling = value
// 		return nil
// 	}
// }

func WithMaxResends(num int) ManagerOption {
	return func(m *Manager) error {
		m.maxResends = num
		return nil
	}
}

func WithProxyAddrPort(a netip.AddrPort) ManagerOption {
	return func(m *Manager) error {
		if !a.IsValid() {
			return ErrProxyAddressNotValid
		}

		m.proxyAddress = net.UDPAddrFromAddrPort(a)
		return nil
	}
}

func WithPublicAddrPort(a netip.AddrPort) ManagerOption {
	return func(m *Manager) error {
		m.publicAddrPort = a
		return nil
	}
}

func WithPublicAddrPortString(s string) ManagerOption {
	return func(m *Manager) error {
		a, err := netip.ParseAddrPort(s)
		if err != nil {
			return err
		}
		m.publicAddrPort = a
		return nil
	}
}

func WithRawTrace(val bool) ManagerOption {
	return func(m *Manager) error {
		m.rawTrace = val
		return nil
	}
}

func WithResendInterval(interval time.Duration) ManagerOption {
	return func(m *Manager) error {
		m.resendInterval = interval
		return nil
	}
}

func WithResendIntervalMilliseconds(interval int) ManagerOption {
	return func(m *Manager) error {
		m.resendInterval = time.Duration(interval) * time.Millisecond
		return nil
	}
}

func WithTimestampTags(val bool) ManagerOption {
	return func(m *Manager) error {
		m.timestampTagging = val
		return nil
	}
}

func WithUserAgent(ua string) ManagerOption {
	return func(m *Manager) error {
		m.userAgent = ua
		return nil
	}
}

func WithGroupLogger(logger *slog.Logger, groupName string) ManagerOption {
	return func(m *Manager) error {
		if groupName != "" {
			logger = logger.WithGroup(groupName)
		}
		m.logger = logger
		return nil
	}
}
