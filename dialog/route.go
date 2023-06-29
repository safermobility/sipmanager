package dialog

import (
	"errors"
	"net"
	"strconv"

	"github.com/safermobility/sipmanager/sip"
	"github.com/safermobility/sipmanager/util"
	"go.uber.org/zap"
)

type AddressRoute struct {
	Address string
	Next    *AddressRoute
}

// Fill in any missing message fields
func (m *Manager) PopulateMessage(via *sip.Via, contact *sip.Addr, msg *sip.Msg) {
	if !msg.IsResponse() {
		if msg.Via == nil {
			msg.Via = via
		}
		if msg.Contact == nil {
			msg.Contact = contact
		}
		if msg.To == nil {
			msg.To = &sip.Addr{Uri: msg.Request}
		}
		if msg.From == nil {
			msg.From = msg.Contact.Copy()
			msg.From.Uri.Param = nil
		}
		if msg.CallID == "" {
			msg.CallID = sip.CallID(util.GenerateCallID())
		}
		if msg.CSeq == 0 {
			msg.CSeq = util.GenerateCSeq()
		}
		if msg.CSeqMethod == "" {
			msg.CSeqMethod = msg.Method
		}
		if msg.MaxForwards == 0 {
			msg.MaxForwards = 70
		}
		if msg.UserAgent == "" {
			msg.UserAgent = m.userAgent
		}
		if msg.Via.Param.Get("branch") == nil {
			msg.Via.Param = &sip.Param{
				Name:  "branch",
				Value: util.GenerateBranch(),
				Next:  msg.Via.Param,
			}
		}
		if msg.From.Param.Get("tag") == nil {
			msg.From.Param = &sip.Param{
				Name:  "tag",
				Value: util.GenerateTag(),
				Next:  msg.From.Param,
			}
		}
	}
}

func RouteMessage(via *sip.Via, contact *sip.Addr, msg *sip.Msg) (host string, port uint16, err error) {
	if msg.IsResponse() {
		if via.CompareHostPort(msg.Via) {
			msg.Via = msg.Via.Next
		}

		host, port = msg.Via.Host, msg.Via.Port
		if received := msg.Via.Param.Get("received"); received != nil {
			host = received.Value
		}

		// fix: Get real port from rport field.
		// Request path like UAC->NAT->UAS will change port(according to NAT type) sometime,
		// we should use rport as real port in request-line
		if rport := msg.Via.Param.Get("rport"); rport != nil && len(rport.Value) > 0 {

			i, err := strconv.Atoi(rport.Value)
			if err != nil {
				return "", 0, err
			}
			port = uint16(i)
		}
	} else {
		if contact.CompareHostPort(msg.Route) {
			msg.Route = msg.Route.Next
		}
		if msg.Route != nil {
			if msg.Method == "REGISTER" {
				return "", 0, errors.New("Don't route REGISTER requests")
			}
			if msg.Route.Uri.Param.Get("lr") != nil {
				// RFC3261 16.12.1.1 Basic SIP Trapezoid
				host, port = msg.Route.Uri.Host, msg.Route.Uri.Port
			} else {
				// RFC3261 16.12.1.2: Traversing a Strict-Routing Proxy
				msg.Route = msg.Route.Copy()
				msg.Route.Last().Next = &sip.Addr{Uri: msg.Request}
				msg.Request = msg.Route.Uri
				msg.Route = msg.Route.Next
				host, port = msg.Request.Host, msg.Request.Port
			}
		} else {
			host, port = msg.Request.Host, msg.Request.Port
		}
	}
	return
}

func (m *Manager) RouteAddress(host string, port uint16, wantSRV bool) (routes *AddressRoute, err error) {
	if net.ParseIP(host) != nil {
		return &AddressRoute{Address: net.JoinHostPort(host, util.Portstr(util.Or5060(port)))}, nil
	}
	if port == 0 {
		if wantSRV {
			_, srvs, err := net.LookupSRV("sip", "udp", host)
			if err == nil && len(srvs) > 0 {
				var serviceAddrs []string
				for i := len(srvs) - 1; i >= 0; i-- {
					routes = &AddressRoute{
						Address: net.JoinHostPort(srvs[i].Target, util.Portstr(srvs[i].Port)),
						Next:    routes,
					}
					serviceAddrs = append(serviceAddrs, routes.Address)
				}
				m.logger.Debug(
					"found route to service",
					zap.String("host", host),
					zap.Strings("service", serviceAddrs),
				)
				return routes, nil
			}
			m.logger.Error(
				"unable to look up SIP/UDP service records",
				zap.Error(err),
				zap.String("host", host),
			)
		}
		port = 5060
	}
	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, util.Portstr(port)))
	if err != nil {
		return nil, err
	}
	return &AddressRoute{Address: addr.String()}, nil
}
