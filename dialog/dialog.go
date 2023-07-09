package dialog

import (
	"errors"
	"fmt"
	"time"

	"github.com/safermobility/sipmanager/sdp"
	"github.com/safermobility/sipmanager/sip"
	"github.com/safermobility/sipmanager/util"
	"golang.org/x/exp/slog"
)

type Status int

const (
	StatusProceeding Status = iota + 1
	StatusRinging
	StatusAnswered
	StatusHangup
	StatusFailed
)

// The "public" interface of a SIP dialog
type Dialog struct {
	OnErr   <-chan error
	OnState <-chan Status
	OnPeer  <-chan *sdp.SDP

	doHangup   chan<- struct{}
	hangupDone bool
}

// The "internal" interface of a SIP dialog
type dialogState struct {
	manager         *Manager
	errChan         chan<- error
	stateChan       chan<- Status
	peerChan        chan<- *sdp.SDP
	hangupChan      <-chan struct{}
	state           Status           // Current state of the dialog.
	callID          sip.CallID       // The Call-ID header value to use for this dialog
	dest            string           // Destination hostname (or IP).
	addr            string           // Destination ip:port.
	routes          *AddressRoute    // List of SRV addresses to attempt contacting, if not using a proxy.
	invite          *sip.Msg         // Our INVITE that established the dialog.
	remote          *sip.Msg         // Message from remote UA that established dialog.
	request         *sip.Msg         // Current outbound request message.
	requestResends  int              // Number of resends of message so far.
	requestTimer    <-chan time.Time // Resend timer for message.
	response        *sip.Msg         // Current outbound request message.
	responseResends int              // Number of resends of message so far.
	responseTimer   <-chan time.Time // Resend timer for message.
	lSeq            int              // Local CSeq value.
	rSeq            int              // Remote CSeq value.
}

// Create a new SIP dialog record and send the INVITE
func (m *Manager) NewDialog(invite *sip.Msg) (*Dialog, error) {
	errChan := make(chan error)
	stateChan := make(chan Status)
	peerChan := make(chan *sdp.SDP)
	hangupChan := make(chan struct{})

	var callID sip.CallID
	if invite.CallID == "" {
		callID = sip.CallID(util.GenerateCallID())
		invite.CallID = callID
	} else {
		callID = invite.CallID
	}

	dls := &dialogState{
		manager:    m,
		errChan:    errChan,
		stateChan:  stateChan,
		peerChan:   peerChan,
		callID:     callID,
		invite:     invite,
		hangupChan: hangupChan,
	}
	go dls.run()

	m.dialogs[callID] = dls

	return &Dialog{
		OnErr:    errChan,
		OnState:  stateChan,
		OnPeer:   peerChan,
		doHangup: hangupChan,
	}, nil
}

// Handle a SIP response message that was received from the remote side
func (dls *dialogState) handleResponse(msg *sip.Msg) bool {
	if !ResponseMatch(dls.request, msg) {
		dls.manager.logger.Warn(
			"received response doesn't match transaction",
			slog.String("original_request", dls.request.String()),
			slog.String("msg", msg.String()),
		)
		return true
	}

	if msg.Status >= sip.StatusOK && dls.request.Method == sip.MethodInvite {
		if msg.Contact == nil {
			dls.errChan <- errors.New("Remote UA sent >=200 response w/o Contact")
			return false
		}
		if err := dls.manager.Send(dls.manager.NewAck(msg, dls.request)); err != nil {
			dls.manager.logger.Error(
				"unable to send ACK message",
				util.SlogError(err),
				slog.String("msg", msg.String()),
			)
			dls.errChan <- fmt.Errorf("unable to send ACK message: %w", err)
			return false
		}
	}

	if msg.Status <= sip.StatusOK {
		dls.checkSDP(msg)
	}

	dls.routes = nil
	// If we got a response to our last message, we probably do not want to resend it.
	// However, we cannot get rid of it yet because we may receive multiple responses (such as `Trying` then `Ringing`).
	dls.requestTimer = nil
	switch msg.Status {
	case sip.StatusTrying:
		dls.transition(StatusProceeding)
	case sip.StatusRinging, sip.StatusSessionProgress:
		dls.transition(StatusRinging)
	case sip.StatusOK:
		switch msg.CSeqMethod {
		case sip.MethodInvite:
			if dls.remote == nil {
				dls.transition(StatusAnswered)
			}
			dls.remote = msg
		case sip.MethodBye, sip.MethodCancel:
			dls.transition(StatusHangup)
			return false
		}
	case sip.StatusServiceUnavailable:
		if dls.request == dls.invite {
			dls.manager.logger.Error(
				"received '503 Service Unavailable' reply to 'INVITE'",
				slog.String("packet", msg.String()),
				slog.String("invite", dls.invite.String()),
				slog.String("dest", dls.dest),
				slog.String("addr", dls.addr),
			)
			return dls.popRoute()
		} else {
			dls.errChan <- &sip.ResponseError{Msg: msg}
			return false
		}
	case sip.StatusMovedPermanently, sip.StatusMovedTemporarily:
		dls.invite.Request = msg.Contact.Uri
		dls.invite.Route = nil
		return dls.sendRequest(dls.invite)
	default:
		if msg.Status > sip.StatusOK {
			dls.errChan <- &sip.ResponseError{Msg: msg}
			return false
		}
	}
	return true
}

// Handle a SIP request message that was received from the remote side
func (dls *dialogState) handleRequest(msg *sip.Msg) bool {
	if msg.MaxForwards <= 0 {
		if err := dls.manager.Send(dls.manager.NewResponse(msg, sip.StatusTooManyHops)); err != nil {
			dls.manager.logger.Error(
				"unable to send '483 Too Many Hops' reply to incoming message",
				util.SlogError(err),
				slog.String("packet", msg.String()),
			)
			return false
		}
		dls.errChan <- errors.New("Remote loop detected")
		return false
	}

	if dls.rSeq == 0 {
		dls.rSeq = msg.CSeq
	} else {
		if msg.CSeq < dls.rSeq {
			// RFC 3261 mandates a 500 response for out of order requests.
			if err := dls.manager.Send(dls.manager.NewResponse(msg, sip.StatusInternalServerError)); err != nil {
				dls.manager.logger.Error(
					"unable to send '500 Internal Server Error' reply to incoming out-of-sequence message",
					util.SlogError(err),
					slog.String("packet", msg.String()),
				)
				return false
			}
			return true
		}
		dls.rSeq = msg.CSeq
	}

	switch msg.Method {
	case sip.MethodBye:
		if err := dls.manager.Send(dls.manager.NewResponse(msg, sip.StatusOK)); err != nil {
			dls.manager.logger.Error(
				"unable to send '200 OK' reply to incoming 'BYE' message",
				util.SlogError(err),
				slog.String("packet", msg.String()),
			)
			return false
		}
		dls.transition(StatusHangup)
		return false
	case sip.MethodOptions: // Probably a keep-alive ping.
		if err := dls.manager.Send(dls.manager.NewResponse(msg, sip.StatusOK)); err != nil {
			dls.manager.logger.Error(
				"unable to send '200 OK' reply to incoming 'OPTIONS' message",
				util.SlogError(err),
				slog.String("packet", msg.String()),
			)
			return false
		}
		return true
	case sip.MethodInvite: // Re-INVITEs are used to change the RTP or signalling path.
		dls.remote = msg
		dls.checkSDP(msg)
		return dls.sendResponse(dls.manager.NewResponse(msg, sip.StatusOK))
	case sip.MethodAck: // Re-INVITE response has been ACK'd.
		dls.response = nil
		dls.responseTimer = nil
		return true
	default:
		if err := dls.manager.Send(dls.manager.NewResponse(msg, sip.StatusMethodNotAllowed)); err != nil {
			dls.manager.logger.Error(
				"unable to send '405 Method Not Allowed' reply to incoming message",
				util.SlogError(err),
				slog.String("packet", msg.String()),
			)
			return false
		}
		return true
	}
}

// If this message has an SDP payload, pass it back to the application
func (dls *dialogState) checkSDP(msg *sip.Msg) {
	if payload, ok := msg.Payload.(*sdp.SDP); ok {
		dls.peerChan <- payload
	}
}

// Send the INVITE and run the loop that handles this dialog's resend timers
func (dls *dialogState) run() {
	defer dls.cleanup()
	if !dls.sendRequest(dls.invite) {
		return
	}

	// This loop handles re-sending non-ACK'ed messages and hangup requests
	// It ends if there are errors doing so
	for {
		select {
		case <-dls.requestTimer:
			if !dls.resendRequest() {
				return
			}
		case <-dls.responseTimer:
			if !dls.resendResponse() {
				return
			}
		case <-dls.hangupChan:
			if !dls.hangup() {
				return
			}
		}

		// If the state is "terminated" or "failed", the `BYE` has
		// been ACK'ed so there will be no further communication
		if dls.state >= StatusHangup {
			return
		}
	}
}

// Prepares to send an INVITE or BYE message - saves it for retrying, and determines the route
func (dls *dialogState) sendRequest(request *sip.Msg) bool {
	host, port, err := RouteMessage(nil, nil, request)
	if err != nil {
		dls.errChan <- err
		return false
	}
	wantSRV := dls.state < StatusAnswered
	routes, err := dls.manager.RouteAddress(host, port, wantSRV)
	if err != nil {
		dls.errChan <- err
		return false
	}
	dls.request = request
	dls.routes = routes
	dls.dest = host
	return dls.popRoute()
}

// Checks whether the route to the destination is valid, and updates the dialog state
// with the new route, if needed.
func (dls *dialogState) popRoute() bool {
	if dls.routes == nil {
		dls.errChan <- errors.New("Failed to contact: " + dls.dest)
		return false
	}
	dls.addr = dls.routes.Address
	dls.routes = dls.routes.Next
	if !dls.connect() {
		return dls.popRoute()
	}
	dls.populate(dls.request)
	if dls.state < StatusAnswered {
		dls.rSeq = 0
		dls.remote = nil
		dls.lSeq = dls.request.CSeq
	}
	dls.requestResends = 0
	dls.requestTimer = time.After(dls.manager.resendInterval)
	if err := dls.manager.Send(dls.request); err != nil {
		dls.manager.logger.Error(
			"error sending request message",
			slog.Int("resends", dls.requestResends),
			slog.String("packet", dls.request.String()),
		)
		return false
	}
	return true
}

func (dls *dialogState) connect() bool {
	// TODO: right now this just assumes that the connection will work,
	//       because we only support using a proxy.
	//       In the future we will allow direct communication too.

	return true
}

func (dls *dialogState) populate(msg *sip.Msg) {
	lHost := dls.manager.PublicAddress().String()
	lPort := dls.manager.PublicPort()

	if msg.Via == nil {
		msg.Via = &sip.Via{Host: lHost}
	}
	msg.Via.Port = lPort
	branch := msg.Via.Param.Get("branch")
	if branch != nil {
		branch.Value = util.GenerateBranch()
	} else {
		msg.Via.Param = &sip.Param{
			Name:  "branch",
			Value: util.GenerateBranch(),
			Next:  msg.Via.Param,
		}
	}

	if msg.Contact == nil {
		msg.Contact = &sip.Addr{Uri: &sip.URI{Scheme: "sip", Host: lHost}}
	}
	msg.Contact.Uri.Port = lPort
	if msg.Contact.Uri.Param.Get("transport") == nil {
		msg.Contact.Uri.Param = &sip.URIParam{
			Name:  "transport",
			Value: "udp",
			Next:  msg.Contact.Uri.Param,
		}
	}

	if msg.Method == sip.MethodInvite {
		if ms, ok := msg.Payload.(*sdp.SDP); ok {
			if ms.Addr == "" {
				ms.Addr = lHost
			}
			if ms.Origin.Addr == "" {
				ms.Origin.Addr = lHost
			}
			if ms.Origin.ID == "" {
				ms.Origin.ID = util.GenerateOriginID()
			}
		}
	}
	dls.manager.PopulateMessage(nil, nil, msg)
}

func (dls *dialogState) resendRequest() bool {
	// If there's nothing to send, or if we explicitly cancelled the resend timer,
	// skip the rest of this and report success.
	if dls.request == nil || dls.requestTimer == nil {
		return true
	}
	if dls.requestResends < dls.manager.maxResends {
		if err := dls.manager.Send(dls.request); err != nil {
			dls.manager.logger.Error(
				"unable to resend message",
				util.SlogError(err),
				slog.String("packet", dls.request.String()),
			)
			return false
		}
		dls.requestResends++
		dls.requestTimer = time.After(dls.manager.resendInterval)
	} else {
		dls.manager.logger.Error(
			"timeout sending request message",
			slog.Int("resends", dls.requestResends),
			slog.String("packet", dls.request.String()),
			slog.String("dest", dls.dest),
			slog.String("addr", dls.addr),
		)
		if !dls.popRoute() {
			return false
		}
	}
	return true
}

// sendResponse is used to reliably send a response to an INVITE only.
func (dls *dialogState) sendResponse(msg *sip.Msg) bool {
	dls.response = msg
	dls.responseResends = 0
	dls.responseTimer = time.After(dls.manager.resendInterval)
	if err := dls.manager.Send(dls.response); err != nil {
		dls.manager.logger.Error(
			"unable to send response to INVITE",
			util.SlogError(err),
			slog.String("packet", msg.String()),
		)
		return false
	}
	return true
}

func (dls *dialogState) resendResponse() bool {
	// If there's nothing to send, or if we explicitly cancelled the resend timer,
	// skip the rest of this and report success.
	if dls.response == nil || dls.responseTimer == nil {
		return true
	}
	if dls.responseResends < dls.manager.maxResends {
		if err := dls.manager.Send(dls.response); err != nil {
			dls.manager.logger.Error(
				"unable to resend response",
				util.SlogError(err),
				slog.String("packet", dls.response.String()),
			)
			return false
		}
		dls.responseResends++
		dls.responseTimer = time.After(dls.manager.resendInterval)
	} else {
		// TODO(jart): If resending INVITE 200 OK, start sending BYE.
		dls.manager.logger.Error(
			"timeout sending response message",
			slog.Int("resends", dls.responseResends),
			slog.String("packet", dls.response.String()),
			slog.String("dest", dls.dest),
			slog.String("addr", dls.addr),
		)
		if !dls.popRoute() {
			return false
		}
	}
	return true
}

func (dls *dialogState) transition(state Status) {
	dls.state = state
	dls.stateChan <- state
}

func (dls *dialogState) cleanup() {
	close(dls.errChan)
	close(dls.stateChan)
	close(dls.peerChan)
	delete(dls.manager.dialogs, dls.callID)
}

func (dls *dialogState) hangup() bool {
	switch dls.state {
	case StatusProceeding, StatusRinging:
		if err := dls.manager.Send(dls.manager.NewCancel(dls.invite)); err != nil {
			dls.manager.logger.Error(
				"unable to send 'CANCEL' message",
				util.SlogError(err),
				slog.String("invite", dls.invite.String()),
			)
			return false
		}
		return true
	case StatusAnswered:
		return dls.sendRequest(dls.manager.NewBye(dls.invite, dls.remote, &dls.lSeq))
	case StatusHangup:
		dls.manager.logger.Error(
			"trying to hang up a call that is already hung up",
		)
		return true
	default:
		//  o  A UA or proxy cannot send CANCEL for a transaction until it gets a
		//     provisional response for the request.  This was allowed in RFC 2543
		//     but leads to potential race conditions.
		dls.transition(StatusHangup)
		return false
	}
}

func (d *Dialog) Hangup() {
	if d.hangupDone {
		return
	}
	d.hangupDone = true
	d.doHangup <- struct{}{}
	close(d.doHangup)
}
