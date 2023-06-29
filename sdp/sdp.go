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
	"strconv"
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
	Origin   Origin      // This must always be present
	Addr     string      // Connect to this IP; never blank (from c=)
	Audio    *Media      // Non-nil if we can establish audio
	Video    *Media      // Non-nil if we can establish video
	Session  string      // s= Session Name (default "-")
	Time     string      // t= Active Time (default "0 0")
	Ptime    int         // Transmit frame every N milliseconds (default 20)
	SendOnly bool        // True if 'a=sendonly' was specified in SDP
	RecvOnly bool        // True if 'a=recvonly' was specified in SDP
	Attrs    [][2]string // a= lines we don't recognize
	Other    [][2]string // Other description
}

// Easy way to create a basic, everyday SDP for VoIP.
func New(addr *net.UDPAddr, codecs ...Codec) *SDP {
	sdp := new(SDP)
	sdp.Addr = addr.IP.String()
	sdp.Origin.ID = util.GenerateOriginID()
	sdp.Origin.Version = sdp.Origin.ID
	sdp.Origin.Addr = sdp.Addr
	sdp.Audio = new(Media)
	sdp.Audio.Proto = "RTP/AVP"
	sdp.Audio.Port = uint16(addr.Port)
	sdp.Audio.Codecs = make([]Codec, len(codecs))
	for i := 0; i < len(codecs); i++ {
		sdp.Audio.Codecs[i] = codecs[i]
	}
	sdp.Attrs = make([][2]string, 0, 8)
	return sdp
}

// parses sdp message text into a happy data structure
func Parse(s string, strict bool) (sdp *SDP, err error) {
	sdp = new(SDP)
	sdp.Session = "-"
	sdp.Time = "0 0"

	// Eat version.
	if !strings.HasPrefix(s, "v=0\r\n") {
		return nil, fmt.Errorf("%w: sdp must start with v=0", ErrInvalidSDP)
	}
	s = s[5:]

	// Turn into lines.
	lines := strings.Split(s, "\r\n")
	if lines == nil || len(lines) < 2 {
		return nil, fmt.Errorf("%w: too few lines in sdp", ErrInvalidSDP)
	}

	// We abstract the structure of the media lines so we need a place to store
	// them before assembling the audio/video data structures.
	var audioinfo, videoinfo string
	var rtpmapList []string
	var fmtpList []string
	sdp.Attrs = make([][2]string, 0, len(lines))

	foundWarnings := false
	warning := fmt.Errorf("%w; ", WarnMalformedSDP)

	// Extract information from SDP.
	var okOrigin, okConn bool
	for _, line := range lines {
		switch {
		case line == "":
			continue
		case len(line) < 3 || line[1] != '=': // empty or invalid line
			if strict {
				return nil, fmt.Errorf("%w: invalid line '%s'", ErrInvalidSDP, line)
			} else {
				foundWarnings = true
				warning = fmt.Errorf("%w; invalid line '%s'", warning, line)
			}
			continue
		case line[0] == 'm': // media line
			line = line[2:]
			if strings.HasPrefix(line, "audio ") {
				audioinfo = line[6:]
			} else if strings.HasPrefix(line, "video ") {
				videoinfo = line[6:]
			} else {
				if strict {
					return nil, fmt.Errorf("%w: unsupported media line '%s'", ErrInvalidSDP, line)
				} else {
					foundWarnings = true
					warning = fmt.Errorf("%w; unsupported media line '%s'", warning, line)
				}
			}
		case line[0] == 's': // session line
			sdp.Session = line[2:]
		case line[0] == 't': // active time
			sdp.Time = line[2:]
		case line[0] == 'c': // connect to this ip address
			if okConn {
				if strict {
					return nil, fmt.Errorf("%w: extra c= line '%s'", ErrInvalidSDP, line)
				} else {
					foundWarnings = true
					warning = fmt.Errorf("%w; dropping extra c= line '%s'", warning, line)
				}
				continue
			}
			sdp.Addr, err = parseConnLine(line)
			if err != nil {
				return nil, err
			}
			okConn = true
		case line[0] == 'o': // origin line
			err = parseOriginLine(&sdp.Origin, line)
			if err != nil {
				return nil, err
			}
			okOrigin = true
		case line[0] == 'a': // attribute lines
			line = line[2:]
			switch {
			case strings.HasPrefix(line, "rtpmap:"):
				rtpmapList = append(rtpmapList, line[7:])
			case strings.HasPrefix(line, "fmtp:"):
				fmtpList = append(fmtpList, line[5:])
			case strings.HasPrefix(line, "ptime:"):
				ptimeS := line[6:]
				if ptime, err := strconv.Atoi(ptimeS); err == nil && ptime > 0 {
					sdp.Ptime = ptime
				} else {
					if strict {
						return nil, fmt.Errorf("%w: invalid ptime value '%s'", ErrInvalidSDP, ptimeS)
					} else {
						foundWarnings = true
						warning = fmt.Errorf("%w; invalid ptime value '%s'", warning, ptimeS)
					}
				}
			case line == "sendrecv":
			case line == "sendonly":
				sdp.SendOnly = true
			case line == "recvonly":
				sdp.RecvOnly = true
			default:
				if n := strings.Index(line, ":"); n >= 0 {
					if n == 0 {
						if strict {
							return nil, fmt.Errorf("%w: evil attribute '%s'", ErrInvalidSDP, line)
						} else {
							foundWarnings = true
							warning = fmt.Errorf("%w; evil attribute '%s'", warning, line)
						}
					} else {
						l := len(sdp.Attrs)
						sdp.Attrs = sdp.Attrs[0 : l+1]
						sdp.Attrs[l] = [2]string{line[0:n], line[n+1:]}
					}
				} else {
					l := len(sdp.Attrs)
					sdp.Attrs = sdp.Attrs[0 : l+1]
					sdp.Attrs[l] = [2]string{line, ""}
				}
			}
		default:

			// Other unknown fields will be saved here
			if n := strings.Index(line, "="); n >= 0 {
				if n == 0 {
					if strict {
						return nil, fmt.Errorf("%w: evil field '%s'", ErrInvalidSDP, line)
					} else {
						foundWarnings = true
						warning = fmt.Errorf("%w; evil field '%s'", warning, line)
					}
				} else {
					sdp.Other = append(sdp.Other, [2]string{line[0:n], line[n+1:]})
				}
			} else {
				sdp.Other = append(sdp.Other, [2]string{line, ""})
			}
		}
	}

	if !okConn || !okOrigin {
		return nil, fmt.Errorf("%w: sdp missing mandatory information", ErrInvalidSDP)
	}

	// Assemble audio/video information.
	var pts []uint8

	if audioinfo != "" {
		var audioPort uint16
		var audioProto string
		audioPort, audioProto, pts, err = parseMediaInfo(audioinfo)
		if audioPort != 0 {
			sdp.Audio = new(Media)
			sdp.Audio.Port, sdp.Audio.Proto = audioPort, audioProto
			if err != nil {
				return nil, err
			}
			err = populateCodecs(sdp.Audio, pts, rtpmapList, fmtpList)
			if err != nil {
				return nil, err
			}
		}
	} else {
		sdp.Video = nil
	}

	if videoinfo != "" {
		var videoPort uint16
		var videoProto string
		videoPort, videoProto, pts, err = parseMediaInfo(videoinfo)
		if videoPort != 0 {
			sdp.Video = new(Media)
			sdp.Video.Port, sdp.Video.Proto = videoPort, videoProto
			if err != nil {
				return nil, err
			}
			err = populateCodecs(sdp.Video, pts, rtpmapList, fmtpList)
			if err != nil {
				return nil, err
			}
		}
	} else {
		sdp.Video = nil
	}

	if sdp.Audio == nil && sdp.Video == nil {
		return nil, fmt.Errorf("%w: sdp has no audio or video information", ErrInvalidSDP)
	}

	if foundWarnings {
		return sdp, warning
	}

	return sdp, nil
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
	if sdp.Audio != nil {
		sdp.Audio.Append("audio", b)
	}
	if sdp.Video != nil {
		sdp.Video.Append("video", b)
	}
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
	if sdp.Ptime > 0 {
		b.WriteString("a=ptime:")
		b.WriteString(strconv.Itoa(sdp.Ptime))
		b.WriteString("\r\n")
	}
	if sdp.SendOnly {
		b.WriteString("a=sendonly\r\n")
	} else if sdp.RecvOnly {
		b.WriteString("a=recvonly\r\n")
	} else {
		b.WriteString("a=sendrecv\r\n")
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
}

// Here we take the list of payload types from the m= line (e.g. 9 18 0 101)
// and turn them into a list of codecs.
//
// If we couldn't find a matching rtpmap, iana standardized will be filled in
// like magic.
func populateCodecs(media *Media, pts []uint8, rtpmapList, fmtpList []string) (err error) {
	media.Codecs = make([]Codec, len(pts))
	for n, pt := range pts {
		codec := &media.Codecs[n]
		codec.PT = pt
		prefix := strconv.FormatInt(int64(pt), 10) + " "
		for _, rtpmap := range rtpmapList {
			if strings.HasPrefix(rtpmap, prefix) {
				err = parseRtpmapInfo(codec, rtpmap[len(prefix):])
				if err != nil {
					return err
				}
				break
			}
		}
		if codec.Name == "" {
			if isDynamicPT(pt) {
				return fmt.Errorf("%w: dynamic codec '%d' missing rtpmap", ErrInvalidSDP, pt)
			} else {
				if v, ok := StandardCodecs[pt]; ok {
					*codec = v
				} else {
					return fmt.Errorf("%w: unknown iana codec id: %d", ErrInvalidSDP, pt)
				}
			}
		}
		for _, fmtp := range fmtpList {
			if strings.HasPrefix(fmtp, prefix) {
				codec.Fmtp = fmtp[len(prefix):]
				break
			}
		}
	}
	return nil
}

// Returns true if IANA says this payload type is dynamic.
func isDynamicPT(pt uint8) bool {
	return (pt >= 96)
}

// Give me the part of the a=rtpmap line that looks like: "PCMU/8000" or
// "L16/16000/2".
func parseRtpmapInfo(codec *Codec, s string) (err error) {
	tokens := strings.Split(s, "/")
	if tokens != nil && len(tokens) >= 2 {
		codec.Name = tokens[0]
		codec.Rate, err = strconv.Atoi(tokens[1])
		if err != nil {
			return fmt.Errorf("%w: invalid rtpmap rate", ErrInvalidSDP)
		}
		if len(tokens) >= 3 {
			codec.Param = tokens[2]
		}
	} else {
		return fmt.Errorf("%w: invalid rtpmap", ErrInvalidSDP)
	}
	return nil
}

// Give me the part of an "m=" line that looks like: "30126 RTP/AVP 0 101".
func parseMediaInfo(s string) (port uint16, proto string, pts []uint8, err error) {
	tokens := strings.Split(s, " ")
	if tokens == nil || len(tokens) < 3 {
		return 0, "", nil, fmt.Errorf("%w: invalid m= line", ErrInvalidSDP)
	}

	// We don't care if they say like "666/2" which is a weird way of saying hey!
	// send ME rtcp too (I think).
	portS := tokens[0]
	if n := strings.Index(portS, "/"); n > 0 {
		portS = portS[0:n]
	}

	// Convert port to int and check range.
	portU, err := strconv.ParseUint(portS, 10, 16)
	if err != nil || !(0 <= port && port <= 65535) {
		return 0, "", nil, fmt.Errorf("%w: invalid m= port: %w", ErrInvalidSDP, err)
	}
	port = uint16(portU)

	// Is it rtp? srtp? udp? tcp? etc. (must be 3+ chars)
	proto = tokens[1]

	// The rest of these tokens are payload types sorted by preference.
	pts = make([]uint8, len(tokens)-2)
	for n, pt := range tokens[2:] {
		pt, err := strconv.ParseUint(pt, 10, 8)
		if err != nil {
			return 0, "", nil, fmt.Errorf("%w: invalid pt in m= line: %w", ErrInvalidSDP, err)
		}
		pts[n] = byte(pt)
	}

	return port, proto, pts, nil
}

// I want a string that looks like "c=IN IP4 10.0.0.38".
func parseConnLine(line string) (addr string, err error) {
	tokens := strings.Split(line[2:], " ")
	if tokens == nil || len(tokens) != 3 {
		return "", fmt.Errorf("%w: invalid conn line", ErrInvalidSDP)
	}
	if tokens[0] != "IN" || (tokens[1] != "IP4" && tokens[1] != "IP6") {
		return "", fmt.Errorf("%w: unsupported conn net type", ErrInvalidSDP)
	}
	addr = tokens[2]
	if n := strings.Index(addr, "/"); n >= 0 {
		return "", fmt.Errorf("%w: multicast address in c= line D:", ErrInvalidSDP)
	}
	return addr, nil
}

// I want a string that looks like "o=root 31589 31589 IN IP4 10.0.0.38".
func parseOriginLine(origin *Origin, line string) error {
	tokens := strings.Split(line[2:], " ")
	if tokens == nil || len(tokens) != 6 {
		return fmt.Errorf("%w: invalid origin line", ErrInvalidSDP)
	}
	if tokens[3] != "IN" || (tokens[4] != "IP4" && tokens[4] != "IP6") {
		return fmt.Errorf("%w: unsupported origin net type", ErrInvalidSDP)
	}
	origin.User = tokens[0]
	origin.ID = tokens[1]
	origin.Version = tokens[2]
	origin.Addr = tokens[5]
	if n := strings.Index(origin.Addr, "/"); n >= 0 {
		return fmt.Errorf("%w: multicast address in o= line D:", ErrInvalidSDP)
	}
	return nil
}
