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
)

// Codec describes one of the codec lines in an SDP. This data will be
// magically filled in if the rtpmap wasn't provided (assuming it's a well
// known codec having a payload type less than 96.)
type Codec struct {
	PT    uint8  // 7-bit payload type we need to put in our RTP packets
	Name  string // e.g. PCMU, G729, telephone-event, etc.
	Rate  int    // frequency in hertz.  usually 8000
	Param string // sometimes used to specify number of channels
	Fmtp  string // some extra info; i.e. dtmf might set as "0-16"

	valid bool // an rtpmap line was parsed for this codec, if needed
}

func NewCodec(pt uint8) (*Codec, error) {
	if isDynamicPT(pt) {
		return &Codec{
			PT: pt,
		}, nil
	}

	if c, ok := StandardCodecs[pt]; ok {
		return &c, nil
	}

	return nil, fmt.Errorf("unknown iana codec id '%d'", pt)
}

func (codec *Codec) Append(b *bytes.Buffer) {
	b.WriteString("a=rtpmap:")
	b.WriteString(strconv.FormatInt(int64(codec.PT), 10))
	b.WriteString(" ")
	b.WriteString(codec.Name)
	b.WriteString("/")
	b.WriteString(strconv.Itoa(codec.Rate))
	if codec.Param != "" {
		b.WriteString("/")
		b.WriteString(codec.Param)
	}
	b.WriteString("\r\n")
	if codec.Fmtp != "" {
		b.WriteString("a=fmtp:")
		b.WriteString(strconv.FormatInt(int64(codec.PT), 10))
		b.WriteString(" ")
		b.WriteString(codec.Fmtp)
		b.WriteString("\r\n")
	}
}

func (codec *Codec) addRtpmap(s string) (err error) {
	// TODO: do we need to check if it's already been set once?
	tokens := strings.Split(s, "/")
	if tokens != nil && len(tokens) >= 2 {
		codec.Name = tokens[0]
		codec.Rate, err = strconv.Atoi(tokens[1])
		if err != nil {
			return fmt.Errorf("invalid rtpmap rate '%s'", tokens[1])
		}
		if len(tokens) >= 3 {
			codec.Param = tokens[2]
		}
		codec.valid = true
	} else {
		return fmt.Errorf("invalid rtpmap '%s'", s)
	}
	return nil
}

func (codec *Codec) addFmtp(s string) (err error) {
	// TODO: do we need to check if it's already been set once?
	codec.Fmtp = s
	return nil
}

// If this codec is dynamic, it must have an rtpmap line present.
// If it is static, an rtpmap line is not required
func (codec *Codec) IsValid() bool {
	if isDynamicPT(codec.PT) {
		return codec.valid
	}
	return true
}

// Returns true if IANA says this payload type is dynamic.
func isDynamicPT(pt uint8) bool {
	return (pt >= 96)
}
