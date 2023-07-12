package dialog

import (
	"bytes"
	"errors"
	"net"
	"strconv"

	"github.com/safermobility/sipmanager/sip"
	"github.com/safermobility/sipmanager/util"
	"golang.org/x/exp/slog"
)

var (
	ErrLocalLoopDetected = errors.New("local loop detected - maxForwards exceeded")
)

func (m *Manager) Send(msg *sip.Msg) error {
	m.PopulateMessage(m.via, m.contact, msg)

	var destination *net.UDPAddr
	if m.proxyAddress != nil {
		destination = m.proxyAddress
	} else {
		host, port, err := RouteMessage(m.via, m.contact, msg)
		if err != nil {
			return err
		}
		addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, strconv.Itoa(int(port))))
		if err != nil {
			return err
		}
		destination = addr
	}

	if msg.MaxForwards > 0 {
		msg.MaxForwards--
		// Note: only check for Max-Forwards reaching zero if it was set non-zero before
		if msg.MaxForwards == 0 {
			return ErrLocalLoopDetected
		}
	}

	m.addTimestamp(msg)

	var b bytes.Buffer
	msg.Append(&b)
	packet := b.Bytes()

	if m.rawTrace {
		m.logger.Debug(
			"outgoing sip packet",
			util.SlogByteString("packet", packet),
			slog.String("destination", destination.String()),
		)
	}

	_, err := m.sock.WriteToUDP(packet, destination)
	if err != nil {
		return err
	}

	return nil
}
