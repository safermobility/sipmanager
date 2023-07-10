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

// Session Description Protocol Library
//
// This is the stuff people embed in SIP packets that tells you how to
// establish audio and/or video sessions.
//
// Here's a typical SDP for a phone call sent by Asterisk:
//
//   v=0
//   o=root 31589 31589 IN IP4 10.0.0.38
//   s=session
//   c=IN IP4 10.0.0.38                <-- ip we should connect to
//   t=0 0
//   m=audio 30126 RTP/AVP 0 101       <-- audio port number and codecs
//   a=rtpmap:0 PCMU/8000              <-- use μ-Law codec at 8000 hz
//   a=rtpmap:101 telephone-event/8000 <-- they support rfc2833 dtmf tones
//   a=fmtp:101 0-16
//   a=silenceSupp:off - - - -         <-- they'll freak out if you use VAD
//   a=ptime:20                        <-- send packet every 20 milliseconds
//   a=sendrecv                        <-- they wanna send and receive audio
//
// Here's an SDP response from MetaSwitch, meaning the exact same
// thing as above, but omitting fields we're smart enough to assume:
//
//   v=0
//   o=- 3366701332 3366701332 IN IP4 1.2.3.4
//   s=-
//   c=IN IP4 1.2.3.4
//   t=0 0
//   m=audio 32898 RTP/AVP 0 101
//   a=rtpmap:101 telephone-event/8000
//   a=ptime:20
//
// If you wanted to go where no woman or man has gone before in the
// voip world, like stream 44.1khz stereo MP3 audio over a IPv6 TCP
// socket for a Flash player to connect to, you could do something
// like:
//
//   v=0
//   o=- 3366701332 3366701332 IN IP6 dead:beef::666
//   s=-
//   c=IN IP6 dead:beef::666
//   t=0 0
//   m=audio 80 TCP/IP 111
//   a=rtpmap:111 MP3/44100/2
//   a=sendonly
//
// Reference Material:
//
// - SDP RFC: http://tools.ietf.org/html/rfc4566
// - SIP/SDP Handshake RFC: http://tools.ietf.org/html/rfc3264
//

package sdp

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/safermobility/sipmanager/util"
)

const (
	ContentType = "application/sdp"
	MaxLength   = 1450
)

var (
	ErrInvalidSDP    = errors.New("invalid sdp")
	WarnMalformedSDP = errors.New("parsing issues in sdp")
)

// SDP represents a Session Description Protocol SIP payload.
type SDP struct {
	Origin    *Origin        // This must always be present
	Addr      string         // Connect to this IP; never blank (from c=)
	Media     []*Media       // Media descriptions, e.g. audio, video
	Session   string         // s= Session Name (default "-")
	Time      string         // t= Active Time (default "0 0")
	Direction MediaDirection // If 'a=sendonly', 'a=recvonly', or 'a=inactive' was specified in SDP
	Attrs     [][2]string    // a= lines we don't recognize
	Other     [][2]string    // Other description
}

// Easy way to create a basic, everyday SDP for VoIP.
func New(addr *net.UDPAddr, codecs ...*Codec) *SDP {
	addrStr := addr.IP.String()
	originID := util.GenerateOriginID()
	sdp := &SDP{
		Addr: addrStr,
		Origin: &Origin{
			ID:      originID,
			Version: originID,
			Addr:    addrStr,
		},
		Media: []*Media{
			{
				Type:   MediaTypeAudio,
				Proto:  "RTP/AVP",
				Port:   uint16(addr.Port),
				Codecs: make([]*Codec, len(codecs)),
			},
		},
	}

	for i := 0; i < len(codecs); i++ {
		sdp.Media[0].Codecs[i] = codecs[i]
	}

	return sdp
}

func (sdp *SDP) ContentType() string {
	return ContentType
}

func (sdp *SDP) Data() []byte {
	if sdp == nil {
		return nil
	}
	var b bytes.Buffer
	sdp.Append(&b)
	return b.Bytes()
}

func (sdp *SDP) String() string {
	if sdp == nil {
		return ""
	}
	var b bytes.Buffer
	sdp.Append(&b)
	return b.String()
}

func (sdp *SDP) Append(b *bytes.Buffer) {
	b.WriteString("v=0\r\n")
	sdp.Origin.Append(b)
	b.WriteString("s=")
	if sdp.Session == "" {
		b.WriteString("-")
	} else {
		b.WriteString(sdp.Session)
	}
	b.WriteString("\r\n")
	if util.IsIPv6(sdp.Addr) {
		b.WriteString("c=IN IP6 ")
	} else {
		b.WriteString("c=IN IP4 ")
	}
	if sdp.Addr == "" {
		// This address from the RFC5735 "TEST-NET-1" block should never route to anywhere.
		b.WriteString("192.0.2.1")
	} else {
		b.WriteString(sdp.Addr)
	}
	b.WriteString("\r\n")
	b.WriteString("t=")
	if sdp.Time == "" {
		b.WriteString("0 0")
	} else {
		b.WriteString(sdp.Time)
	}
	b.WriteString("\r\n")
	for _, attr := range sdp.Attrs {
		if attr[1] == "" {
			b.WriteString("a=")
			b.WriteString(attr[0])
			b.WriteString("\r\n")
		} else {
			b.WriteString("a=")
			b.WriteString(attr[0])
			b.WriteString(":")
			b.WriteString(attr[1])
			b.WriteString("\r\n")
		}
	}
	if sdp.Direction != "" {
		b.WriteString("a=")
		b.WriteString(string(sdp.Direction))
		b.WriteString("\r\n")
	}

	// save unknown field
	if sdp.Other != nil {
		for _, v := range sdp.Other {
			b.WriteString(v[0])
			b.WriteString("=")
			b.WriteString(v[1])
			b.WriteString("\r\n")
		}
	}

	for _, media := range sdp.Media {
		media.Append(b)
	}
}

func (sdp *SDP) addAttribute(line string, strict bool) error {
	lineParts := strings.SplitN(line, ":", 2)

	// See RFC 8866 section 6
	switch lineParts[0] {
	case "tool": // section 6.3
		// TODO: save tool info
	case "sendrecv", "sendonly", "recvonly", "inactive": // section 6.7
		if sdp.Direction != "" {
			if strict {
				return fmt.Errorf("extra media direction line '%s' for session", line)
			} else {
				return fmt.Errorf("dropping extra media direction line '%s' for session", line)
			}
		}
		sdp.Direction = MediaDirection(line)
	case "":
		// empty key, i.e. line started with "a=:"
		return fmt.Errorf("invalid attribute '%s' for media", line)
	default:
		if len(lineParts) == 1 {
			sdp.Attrs = append(sdp.Attrs, [2]string{lineParts[0], ""})
		} else {
			sdp.Attrs = append(sdp.Attrs, [2]string{lineParts[0], lineParts[1]})
		}
	}

	return nil
}

func (sdp *SDP) addOther(line string) error {
	split := strings.SplitN(line, "=", 2)
	if len(split[0]) == 0 { // '=' was the first character
		return fmt.Errorf("invalid attribute '%s' for session", line)
	}

	if len(split) == 1 {
		sdp.Other = append(sdp.Other, [2]string{split[0], ""})
	} else {
		sdp.Other = append(sdp.Other, [2]string{split[0], split[1]})
	}

	return nil
}
