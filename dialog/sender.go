package dialog

import (
	"bytes"
	"errors"
	"net"
	"strconv"

	"github.com/safermobility/sipmanager/sip"
	"go.uber.org/zap"
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
	}
	if msg.MaxForwards == 0 {
		return ErrLocalLoopDetected
	}

	m.addTimestamp(msg)

	var b bytes.Buffer
	msg.Append(&b)
	packet := b.Bytes()

	if m.rawTrace {
		m.logger.Debug(
			"outgoing sip packet",
			zap.ByteString("packet", packet),
			zap.String("destination", destination.String()),
		)
	}

	_, err := m.sock.WriteToUDP(packet, destination)
	if err != nil {
		return err
	}

	return nil
}
