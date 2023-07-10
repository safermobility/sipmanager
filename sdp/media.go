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

package sdp

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/safermobility/sipmanager/util"
)

// Media is a high level representation of the c=/m=/a= lines for describing a
// specific type of media. Only "audio" and "video" are supported at this time.
type Media struct {
	Type      MediaType         // audio, video, text, application, message, etc.
	Proto     TransportProtocol // RTP, SRTP, UDP, UDPTL, TCP, TLS, etc.
	Port      uint16            // Port number (0 - 2^16-1)
	NumPorts  int               // If multiple ports are being used
	Addr      string            // The address from the media-specific `c=` line, if present
	Direction MediaDirection    // sendrecv, sendonly, recvonly, inactive
	Codecs    []*Codec          // Collection of codecs of a specific type.
	Ptime     int               // Transmit frame every N milliseconds (default 20)
	Maxptime  int               // Maximum number of milliseconds per packet (default 20)
	Attrs     [][2]string       // Attributes for this media description
	Other     [][2]string       // Unrecognized properties for this media description
}

// Parse an `m=` line (e.g. "audio 30126 RTP/AVP 0 96") and return a corresponding Media object
func NewMediaFromLine(line string, strict bool) (*Media, error) {
	tokens := strings.Fields(line)

	if tokens == nil || len(tokens) < 4 {
		return nil, fmt.Errorf("not enough tokens in m= line: %d", len(tokens))
	}

	mediaType, ok := IsKnownMediaType(tokens[0])
	if strict && !ok {
		return nil, fmt.Errorf("unsupported media type '%s'", tokens[0])
	}

	var port uint16
	var portCount int
	// Port can be optionally followed by `/num` for a number of ports
	portStr := strings.Split(tokens[1], "/")
	portU, err := strconv.ParseUint(portStr[0], 10, 16)
	if err != nil || !(0 <= portU && portU <= 65535) {
		return nil, fmt.Errorf("invalid m= port '%s'", portStr[0])
	}
	port = uint16(portU)

	// Sending a `0` port means disabled, so return nothing
	if port == 0 {
		return nil, nil
	}

	if len(portStr) > 1 {
		numU, err := strconv.ParseInt(portStr[1], 10, 0)
		if err != nil || !(0 <= numU && numU <= 65535) {
			return nil, fmt.Errorf("invalid m= port range '%s'", portStr[1])
		}
		portCount = int(numU)
	}

	proto, ok := IsKnownTransportProtocol(tokens[2])
	if strict && !ok {
		return nil, fmt.Errorf("unsupported media protocol '%s'", tokens[2])
	}

	// The rest of these tokens are payload types sorted by preference.
	pts := make([]uint8, len(tokens)-3)
	for n, pt := range tokens[3:] {
		pti, err := strconv.ParseUint(pt, 10, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid pt '%s' in m= line: %w", pt, err)
		}
		pts[n] = uint8(pti)
	}

	codecs := make([]*Codec, len(pts))
	for n, pt := range pts {
		codecs[n], err = NewCodec(pt)
		if err != nil {
			return nil, err
		}
	}

	m := &Media{
		Type:     mediaType,
		Port:     port,
		NumPorts: portCount,
		Proto:    proto,
		Codecs:   codecs,
	}

	return m, nil
}

func (media *Media) Append(b *bytes.Buffer) {
	b.WriteString("m=")
	b.WriteString(string(media.Type))
	b.WriteString(" ")
	b.WriteString(strconv.FormatUint(uint64(media.Port), 10))
	if media.NumPorts > 1 {
		b.WriteRune('/')
		b.WriteString(strconv.FormatInt(int64(media.NumPorts), 10))
	}
	b.WriteString(" ")
	if media.Proto == "" {
		b.WriteString("RTP/AVP")
	} else {
		b.WriteString(string(media.Proto))
	}
	for _, codec := range media.Codecs {
		b.WriteString(" ")
		b.WriteString(strconv.FormatInt(int64(codec.PT), 10))
	}
	b.WriteString("\r\n")

	// If this media description has its own `c=` line
	if media.Addr != "" {
		if util.IsIPv6(media.Addr) {
			b.WriteString("c=IN IP6 ")
		} else {
			b.WriteString("c=IN IP4 ")
		}
		b.WriteString(media.Addr)
		b.WriteString("\r\n")
	}

	for _, codec := range media.Codecs {
		codec.Append(b)
	}

	for _, attr := range media.Attrs {
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

	if media.Ptime > 0 {
		b.WriteString("a=ptime:")
		b.WriteString(strconv.Itoa(media.Ptime))
		b.WriteString("\r\n")
	}
	if media.Maxptime > 0 {
		b.WriteString("a=maxptime:")
		b.WriteString(strconv.Itoa(media.Maxptime))
		b.WriteString("\r\n")
	}

	if media.Direction != "" {
		b.WriteString("a=")
		b.WriteString(string(media.Direction))
		b.WriteString("\r\n")
	}
}

func (media *Media) addAttribute(line string, strict bool) error {
	lineParts := strings.SplitN(line, ":", 2)

	// See RFC 8866 section 6
	switch lineParts[0] {
	case "ptime": // section 6.4
		ptimeS := lineParts[1]
		if ptime, err := strconv.Atoi(ptimeS); err == nil && ptime > 0 {
			media.Ptime = ptime
		} else {
			return fmt.Errorf("invalid ptime value '%s'", ptimeS)
		}
	case "maxptime": // section 6.5
		ptimeS := lineParts[1]
		if ptime, err := strconv.Atoi(ptimeS); err == nil && ptime > 0 {
			media.Maxptime = ptime
		} else {
			return fmt.Errorf("invalid maxptime value '%s'", ptimeS)
		}
	case "rtpmap": // section 6.6
		if err := media.addRtpmap(lineParts[1]); err != nil {
			if strict {
				return fmt.Errorf("invalid rtpmap line '%s' for media", line)
			} else {
				return fmt.Errorf("ignoring invalid rtpmap line '%s' for media", line)
			}
		}
	case "sendrecv", "sendonly", "recvonly", "inactive": // section 6.7
		if media.Direction != "" {
			if strict {
				return fmt.Errorf("extra media direction line '%s' for media", line)
			} else {
				return fmt.Errorf("dropping extra media direction line '%s' for media", line)
			}
		}
		media.Direction = MediaDirection(line)
	case "fmtp": // section 6.15
		if err := media.addFmtp(lineParts[1]); err != nil {
			if strict {
				return fmt.Errorf("invalid fmtp line '%s' for media", line)
			} else {
				return fmt.Errorf("ignoring invalid fmtp line '%s' for media", line)
			}
		}
	case "":
		// empty key, i.e. line started with "a=:"
		return fmt.Errorf("invalid attribute '%s' for media", line)
	default:
		if len(lineParts) == 1 {
			media.Attrs = append(media.Attrs, [2]string{lineParts[0], ""})
		} else {
			media.Attrs = append(media.Attrs, [2]string{lineParts[0], lineParts[1]})
		}
	}

	return nil
}

// Give me the part of the a=rtpmap line that looks like: "PCMU/8000" or
// "L16/16000/2" and add it to the codec definition.
func (media *Media) addRtpmap(line string) error {
	tokens := strings.Fields(line)
	payloadType, value := tokens[0], tokens[1]

	pt, err := strconv.ParseUint(payloadType, 10, 8)
	if err != nil {
		return fmt.Errorf("invalid pt '%s' in rtpmap: %w", payloadType, err)
	}

	for _, c := range media.Codecs {
		if uint8(pt) == c.PT {
			c.addRtpmap(value)
			return nil
		}
	}

	return fmt.Errorf("codec id '%s' in rtpmap not found in media description", payloadType)
}

func (media *Media) addFmtp(line string) error {
	tokens := strings.Fields(line)
	payloadType, value := tokens[0], tokens[1]

	pt, err := strconv.ParseUint(payloadType, 10, 8)
	if err != nil {
		return fmt.Errorf("invalid pt '%s' in rtpmap: %w", payloadType, err)
	}

	for _, c := range media.Codecs {
		if uint8(pt) == c.PT {
			c.addFmtp(value)
			return nil
		}
	}

	return fmt.Errorf("codec id '%s' in fmtp not found in media description", payloadType)
}

func (media *Media) addOther(line string) error {
	split := strings.SplitN(line, "=", 2)
	if len(split[0]) == 0 { // '=' was the first character
		return fmt.Errorf("invalid attribute '%s' for media", line)
	}

	if len(split) == 1 {
		media.Other = append(media.Other, [2]string{split[0], ""})
	} else {
		media.Other = append(media.Other, [2]string{split[0], split[1]})
	}

	return nil
}
