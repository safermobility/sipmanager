package sdp

import (
	"fmt"
	"strings"
)

// parses sdp message text into a happy data structure
func Parse(s string, strict bool) (*SDP, error) {
	sdp := &SDP{
		Session: "-",
		Time:    "0 0",
	}

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

	foundWarnings := false
	warning := WarnMalformedSDP

	// We must find one of these before the first `m=` media line
	var foundOrigin, foundConn bool
	// The current media description
	var inMedia *Media
	// If there is an unsupported media line, we need to skip all of its attributes as well
	var skippingInvalidMedia bool

	// Extract information from SDP.
	var err error
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
		case line[0] == 's': // session line
			sdp.Session = line[2:]
		case line[0] == 't': // active time
			sdp.Time = line[2:]
		case line[0] == 'm': // media line
			line = line[2:]
			skippingInvalidMedia = false
			inMedia, err = NewMediaFromLine(line, strict)
			if err != nil {
				if strict {
					return nil, fmt.Errorf("%w: %w - '%s'", ErrInvalidSDP, err, line)
				} else {
					skippingInvalidMedia = true
					foundWarnings = true
					warning = fmt.Errorf("%w; %w - '%s'", warning, err, line)
				}
				continue
			}
			if inMedia == nil {
				skippingInvalidMedia = true
				continue
			}
			sdp.Media = append(sdp.Media, inMedia)
		case line[0] == 'c': // connect to this ip address
			// If we are already in the media descriptions, we need to skip the `c=` line
			// connected with any media we were unable to process
			if skippingInvalidMedia {
				continue
			}
			if inMedia == nil {
				if foundConn {
					if strict {
						return nil, fmt.Errorf("%w: extra c= line '%s' for session", ErrInvalidSDP, line)
					} else {
						foundWarnings = true
						warning = fmt.Errorf("%w; dropping extra c= line '%s' for session", warning, line)
					}
					continue
				}
				sdp.Addr, err = parseConnLine(line)
				if err != nil {
					return nil, err
				}
				foundConn = true
			} else {
				if inMedia.Addr != "" {
					if strict {
						return nil, fmt.Errorf("%w: extra c= line '%s' for media", ErrInvalidSDP, line)
					} else {
						foundWarnings = true
						warning = fmt.Errorf("%w; dropping extra c= line '%s' for media", warning, line)
					}
					continue
				}
				inMedia.Addr, err = parseConnLine(line)
				if err != nil {
					return nil, err
				}
			}
		case line[0] == 'o': // origin line
			if inMedia != nil || skippingInvalidMedia {
				if strict {
					return nil, fmt.Errorf("%w: found o= line '%s' after media", ErrInvalidSDP, line)
				} else {
					foundWarnings = true
					warning = fmt.Errorf("%w; ignoring o= line '%s' after media", warning, line)
				}
				continue
			}
			if sdp.Origin != nil {
				if strict {
					return nil, fmt.Errorf("%w: extra o= line '%s' for session", ErrInvalidSDP, line)
				} else {
					foundWarnings = true
					warning = fmt.Errorf("%w; dropping extra o= line '%s' for session", warning, line)
				}
				continue
			}
			sdp.Origin, err = parseOriginLine(line)
			if err != nil {
				return nil, err
			}
			foundOrigin = true
		case line[0] == 'a': // attribute lines
			// If we are already in the media descriptions, we need to skip the `a=` line
			// connected with any media we were unable to process
			if skippingInvalidMedia {
				continue
			}
			line = line[2:]
			if inMedia == nil {
				if err := sdp.addAttribute(line, strict); err != nil {
					if strict {
						return nil, fmt.Errorf("%w: unable to add attribute to session: %w", ErrInvalidSDP, err)
					} else {
						foundWarnings = true
						warning = fmt.Errorf("%w; dropping unprocessable attribute '%s' for session: %w", warning, line, err)
					}
				}
			} else {
				if err := inMedia.addAttribute(line, strict); err != nil {
					if strict {
						return nil, fmt.Errorf("%w: unable to add attribute to media: %w", ErrInvalidSDP, err)
					} else {
						foundWarnings = true
						warning = fmt.Errorf("%w; dropping unprocessable attribute '%s' for media: %w", warning, line, err)
					}
				}
			}
		default:
			// If we are already in the media descriptions, we need to skip any remaining lines
			// connected with any media we were unable to process
			if skippingInvalidMedia {
				continue
			}
			if inMedia == nil {
				if err := sdp.addOther(line); err != nil {
					if strict {
						return nil, fmt.Errorf("%w: unable to add property to session: %w", ErrInvalidSDP, err)
					} else {
						foundWarnings = true
						warning = fmt.Errorf("%w; dropping unprocessable property '%s' for session: %w", warning, line, err)
					}
				}
			} else {
				if err := inMedia.addOther(line); err != nil {
					if strict {
						return nil, fmt.Errorf("%w: unable to add property to media: %w", ErrInvalidSDP, err)
					} else {
						foundWarnings = true
						warning = fmt.Errorf("%w; dropping unprocessable property '%s' for media: %w", warning, line, err)
					}
				}
			}
		}
	}

	if !foundConn && !foundOrigin {
		return nil, fmt.Errorf("%w: sdp missing mandatory information", ErrInvalidSDP)
	}

	if len(sdp.Media) == 0 {
		return nil, fmt.Errorf("%w: no media descriptions found", ErrInvalidSDP)
	}

	for _, m := range sdp.Media {
		for _, c := range m.Codecs {
			if !c.IsValid() {
				if strict {
					return nil, fmt.Errorf("%w: missing codec rtpmap for codec '%d'", ErrInvalidSDP, c.PT)
				} else {
					foundWarnings = true
					warning = fmt.Errorf("%w: missing codec rtpmap for codec '%d'", warning, c.PT)
				}
			}
		}
	}

	if foundWarnings {
		return sdp, warning
	}

	return sdp, nil
}

// I want a string that looks like "c=IN IP4 10.0.0.38".
func parseConnLine(line string) (addr string, err error) {
	tokens := strings.Fields(line[2:])
	if tokens == nil || len(tokens) != 3 {
		return "", fmt.Errorf("%w: invalid conn line", ErrInvalidSDP)
	}
	if tokens[0] != "IN" || (tokens[1] != "IP4" && tokens[1] != "IP6") {
		return "", fmt.Errorf("%w: unsupported conn net type", ErrInvalidSDP)
	}
	addr = tokens[2]
	if n := strings.Index(addr, "/"); n >= 0 {
		return "", fmt.Errorf("%w: multicast address in c= line", ErrInvalidSDP)
	}
	return addr, nil
}

// I want a string that looks like "o=root 31589 31589 IN IP4 10.0.0.38".
func parseOriginLine(line string) (*Origin, error) {
	tokens := strings.Fields(line[2:])
	if tokens == nil || len(tokens) != 6 {
		return nil, fmt.Errorf("%w: invalid origin line", ErrInvalidSDP)
	}
	if tokens[3] != "IN" || (tokens[4] != "IP4" && tokens[4] != "IP6") {
		return nil, fmt.Errorf("%w: unsupported origin net type", ErrInvalidSDP)
	}
	if n := strings.Index(tokens[5], "/"); n >= 0 {
		return nil, fmt.Errorf("%w: multicast address in o= line", ErrInvalidSDP)
	}
	return &Origin{
		User:    tokens[0],
		ID:      tokens[1],
		Version: tokens[2],
		Addr:    tokens[5],
	}, nil
}
