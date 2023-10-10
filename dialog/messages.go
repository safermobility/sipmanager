// Copyright 2020 Justine Alexandra Roberts Tunney
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dialog

import (
	"github.com/safermobility/sipmanager/sip"
	"golang.org/x/exp/slog"
)

const (
	GosipAllow             = "ACK, CANCEL, BYE, OPTIONS"
	GosipAllowWithReinvite = "INVITE, ACK, CANCEL, BYE, OPTIONS"
)

func (m *Manager) NewResponse(msg *sip.Msg, status int) *sip.Msg {
	allow := GosipAllow
	if m.allowReinvite {
		allow = GosipAllowWithReinvite
	}
	return &sip.Msg{
		Status:      status,
		Phrase:      sip.Phrase(status),
		Via:         msg.Via,
		From:        msg.From,
		To:          msg.To,
		CallID:      msg.CallID,
		CSeq:        msg.CSeq,
		CSeqMethod:  msg.CSeqMethod,
		RecordRoute: msg.RecordRoute,
		UserAgent:   m.userAgent,
		Allow:       allow,
	}
}

// http://tools.ietf.org/html/rfc3261#section-17.1.1.3
func (m *Manager) NewAck(msg, invite *sip.Msg) *sip.Msg {
	return &sip.Msg{
		Method:             sip.MethodAck,
		Request:            msg.Contact.Uri,
		From:               msg.From,
		To:                 msg.To,
		Via:                msg.Via.Detach(),
		CallID:             msg.CallID,
		CSeq:               msg.CSeq,
		CSeqMethod:         "ACK",
		Route:              msg.RecordRoute.Reversed(),
		Authorization:      invite.Authorization,
		ProxyAuthorization: invite.ProxyAuthorization,
		UserAgent:          m.userAgent,
	}
}

func (m *Manager) NewCancel(invite *sip.Msg) *sip.Msg {
	if invite.IsResponse() || invite.Method != sip.MethodInvite {
		m.logger.Error(
			"trying to CANCEL something that is not an INVITE",
			slog.String("invite", invite.String()),
		)
	}

	return &sip.Msg{
		Method:     sip.MethodCancel,
		Request:    invite.Request,
		Via:        invite.Via,
		From:       invite.From,
		To:         invite.To,
		CallID:     invite.CallID,
		CSeq:       invite.CSeq,
		CSeqMethod: sip.MethodCancel,
		Route:      invite.Route,
	}
}

func (m *Manager) NewBye(invite, remote *sip.Msg, lSeq *int) *sip.Msg {
	if lSeq == nil {
		lSeq = new(int)
		*lSeq = invite.CSeq
	}
	*lSeq++
	return &sip.Msg{
		Method:     sip.MethodBye,
		Request:    remote.Contact.Uri,
		From:       invite.From,
		To:         remote.To,
		CallID:     invite.CallID,
		CSeq:       *lSeq,
		CSeqMethod: sip.MethodBye,
		Route:      remote.RecordRoute.Reversed(),
	}
}

// Returns true if `resp` can be considered an appropriate response to `msg`.
// Do not use for ACKs.
func ResponseMatch(req, rsp *sip.Msg) bool {
	return (rsp.IsResponse() &&
		rsp.CSeq == req.CSeq &&
		rsp.CSeqMethod == req.Method &&
		rsp.Via.Last().CompareHostPort(req.Via) &&
		rsp.Via.Last().CompareBranch(req.Via))
}

// Returns true if `ack` can be considered an appropriate response to `msg`.
// We don't enforce a matching Via because some VoIP software will generate a
// new branch for ACKs.
func AckMatch(msg, ack *sip.Msg) bool {
	return (!ack.IsResponse() &&
		ack.Method == sip.MethodAck &&
		ack.CSeq == msg.CSeq &&
		ack.CSeqMethod == sip.MethodAck &&
		ack.Via.Last().CompareHostPort(msg.Via))
}
