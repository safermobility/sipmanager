package dialog

import (
	"fmt"
	"net/netip"
	"strconv"
	"time"

	"github.com/safermobility/sipmanager/sip"
	"go.uber.org/zap"
)

func (m *Manager) ReceiveMessages() {
	buf := make([]byte, 2048)
	for {
		amt, addr, err := m.sock.ReadFromUDPAddrPort(buf)
		if err != nil {
			m.errors <- err
			break
		}
		packet := buf[0:amt]
		if m.rawTrace {
			m.logger.Debug("incoming sip packet", zap.ByteString("packet", packet))
		}
		msg, err := sip.ParseMsg(packet)
		if err != nil {
			m.logger.Warn("unable to parse sip message", zap.Error(err), zap.ByteString("packet", packet))
		}
		m.addReceived(msg, addr)
		m.addTimestamp(msg)
		if msg.Route != nil && m.IsLocalHostPort(msg.Route.Uri) {
			msg.Route = msg.Route.Next
		}
		// TODO what host/port to use here:
		// m.fixMessagesFromStrictRouters()

		m.HandleIncomingMessage(msg)
	}
}

// Check if the incoming message is part of an existing transaction
// and send it to that transaction object to be handled
func (m *Manager) HandleIncomingMessage(msg *sip.Msg) {
	if msg.VersionMajor != 2 || msg.VersionMinor != 0 {
		m.logger.Warn("received unknown SIP version in incoming message", zap.String("version", fmt.Sprintf("%d/%d", msg.VersionMajor, msg.VersionMinor)))
		err := m.Send(m.NewResponse(msg, sip.StatusVersionNotSupported))
		if err != nil {
			m.logger.Error("unable to send '505 Version Not Supported' reply to incoming message", zap.Error(err), zap.String("packet", msg.String()))
		}
		return
	}

	if dlg, ok := m.dialogs[msg.CallID]; ok {
		if msg.IsResponse() {
			dlg.handleResponse(msg)
		} else {
			dlg.handleRequest(msg)
		}
		return
	}

	err := m.Send(m.NewResponse(msg, sip.StatusCallTransactionDoesNotExist))
	m.logger.Warn("received incoming message for unknown transaction", zap.String("call-id", string(msg.CallID)))
	if err != nil {
		m.logger.Error("unable to send '481 Call Transaction Does Not Exist' reply to incoming message", zap.Error(err), zap.String("packet", msg.String()))
	}
}

func (m *Manager) addReceived(msg *sip.Msg, addr netip.AddrPort) {
	if msg.IsResponse() {
		return
	}
	if msg.Via.Port != addr.Port() {

		rport := msg.Via.Param.Get("rport")
		port := fmt.Sprintf("%d", addr.Port())

		if rport == nil {
			msg.Via.Param = &sip.Param{
				Name:  "rport",
				Value: port,
				Next:  msg.Via.Param,
			}
		} else {
			// implied rport is 5060, but some NAT will use another port,we use real port instead
			if len(rport.Value) == 0 {
				rport.Value = port
			}
		}
	}
	ip := addr.Addr().Unmap().String()
	if msg.Via.Host != ip {
		if msg.Via.Param.Get("received") == nil {
			msg.Via.Param = &sip.Param{
				Name:  "received",
				Value: ip,
				Next:  msg.Via.Param,
			}
		}
	}
}

func (m *Manager) addTimestamp(msg *sip.Msg) {
	if m.timestampTagging {
		msg.Via.Param = &sip.Param{
			Name:  "usi",
			Value: strconv.FormatInt(time.Now().UnixMicro(), 10),
			Next:  msg.Via.Param,
		}
	}
}

func (m *Manager) IsLocalHostPort(uri *sip.URI) bool {
	// Make sure to default uri.Port to `5060` if it is currently set to `0` (i.e. not provided)
	messagePort := uri.Port
	if messagePort == 0 {
		messagePort = 5060
	}
	if uri.Host == m.PublicAddress().String() && messagePort == m.PublicPort() {
		return true
	}

	return false
}

// RFC3261 16.4 Route Information Preprocessing
// RFC3261 16.12.1.2: Traversing a Strict-Routing Proxy
func (m *Manager) fixMessagesFromStrictRouters(lHost string, lPort uint16, msg *sip.Msg) {
	if msg.Request != nil &&
		msg.Request.Param.Get("lr") != nil &&
		msg.Route != nil &&
		msg.Request.Host == lHost &&
		msg.Request.GetPort() == lPort {
		var oldReq, newReq *sip.URI
		if msg.Route.Next == nil {
			oldReq, newReq = msg.Request, msg.Route.Uri
			msg.Request = msg.Route.Uri
			msg.Route = nil
		} else {
			seclast := msg.Route
			for ; seclast.Next.Next != nil; seclast = seclast.Next {
			}
			oldReq, newReq = msg.Request, seclast.Next.Uri
			msg.Request = seclast.Next.Uri
			seclast.Next = nil
			msg.Route.Last()
		}
		m.logger.Debug("fixing request URI after strict router traversal", zap.Any("old", oldReq), zap.Any("new", newReq))
	}
}
