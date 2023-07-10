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

package sdp_test

import (
	"fmt"
	"testing"

	"github.com/safermobility/sipmanager/sdp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sdpTest struct {
	name string   // arbitrary name for test
	s    string   // raw sdp input to parse
	s2   string   // non-blank if sdp looks different when we format it
	sdp  *sdp.SDP // memory structure of 's' after parsing
	err  error
}

var sdpTests = []sdpTest{

	{
		name: "Asterisk PCMU+DTMF",
		s: ("v=0\r\n" +
			"o=root 31589 31589 IN IP4 10.0.0.38\r\n" +
			"s=session\r\n" +
			"c=IN IP4 10.0.0.38\r\n" +
			"t=0 0\r\n" +
			"m=audio 30126 RTP/AVP 0 101\r\n" +
			"a=rtpmap:0 PCMU/8000\r\n" +
			"a=rtpmap:101 telephone-event/8000\r\n" +
			"a=fmtp:101 0-16\r\n" +
			"a=silenceSupp:off - - - -\r\n" +
			"a=ptime:20\r\n" +
			"a=sendrecv\r\n"),
		sdp: &sdp.SDP{
			Origin: &sdp.Origin{
				User:    "root",
				ID:      "31589",
				Version: "31589",
				Addr:    "10.0.0.38",
			},
			Session: "session",
			Time:    "0 0",
			Addr:    "10.0.0.38",
			Media: []*sdp.Media{
				{
					Type:      sdp.MediaTypeAudio,
					Proto:     "RTP/AVP",
					Port:      30126,
					Ptime:     20,
					Direction: sdp.SendRecv,
					Codecs: []*sdp.Codec{
						{PT: 0, Name: "PCMU", Rate: 8000},
						{PT: 101, Name: "telephone-event", Rate: 8000, Fmtp: "0-16"},
					},
					Attrs: [][2]string{
						{"silenceSupp", "off - - - -"},
					},
				},
			},
		},
	},

	{
		name: "Audio+Video+Implicit+Fmtp",
		s: "v=0\r\n" +
			"o=- 3366701332 3366701332 IN IP4 1.2.3.4\r\n" +
			"c=IN IP4 1.2.3.4\r\n" +
			"m=audio 32898 RTP/AVP 18\r\n" +
			"a=fmtp:18 annexb=yes\r\n" +
			"m=video 32900 RTP/AVP 34\r\n",
		s2: "v=0\r\n" +
			"o=- 3366701332 3366701332 IN IP4 1.2.3.4\r\n" +
			"s=-\r\n" +
			"c=IN IP4 1.2.3.4\r\n" +
			"t=0 0\r\n" +
			"m=audio 32898 RTP/AVP 18\r\n" +
			"a=rtpmap:18 G729/8000\r\n" +
			"a=fmtp:18 annexb=yes\r\n" +
			"m=video 32900 RTP/AVP 34\r\n" +
			"a=rtpmap:34 H263/90000\r\n",
		sdp: &sdp.SDP{
			Origin: &sdp.Origin{
				User:    "-",
				ID:      "3366701332",
				Version: "3366701332",
				Addr:    "1.2.3.4",
			},
			Addr:    "1.2.3.4",
			Session: "-",
			Time:    "0 0",
			Media: []*sdp.Media{
				{
					Type:  sdp.MediaTypeAudio,
					Proto: "RTP/AVP",
					Port:  32898,
					Codecs: []*sdp.Codec{
						{PT: 18, Name: "G729", Rate: 8000, Fmtp: "annexb=yes"},
					},
				},
				{
					Type:  sdp.MediaTypeVideo,
					Proto: "RTP/AVP",
					Port:  32900,
					Codecs: []*sdp.Codec{
						{PT: 34, Name: "H263", Rate: 90000},
					},
				},
			},
		},
	},

	{
		name: "Implicit Codecs",
		s: "v=0\r\n" +
			"o=- 3366701332 3366701332 IN IP4 1.2.3.4\r\n" +
			"s=-\r\n" +
			"c=IN IP4 1.2.3.4\r\n" +
			"t=0 0\r\n" +
			"m=audio 32898 RTP/AVP 9 18 0 101\r\n" +
			"a=rtpmap:101 telephone-event/8000\r\n" +
			"a=ptime:20\r\n",
		s2: "v=0\r\n" +
			"o=- 3366701332 3366701332 IN IP4 1.2.3.4\r\n" +
			"s=-\r\n" +
			"c=IN IP4 1.2.3.4\r\n" +
			"t=0 0\r\n" +
			"m=audio 32898 RTP/AVP 9 18 0 101\r\n" +
			"a=rtpmap:9 G722/8000\r\n" +
			"a=rtpmap:18 G729/8000\r\n" +
			"a=rtpmap:0 PCMU/8000\r\n" +
			"a=rtpmap:101 telephone-event/8000\r\n" +
			"a=ptime:20\r\n",
		sdp: &sdp.SDP{
			Origin: &sdp.Origin{
				User:    "-",
				ID:      "3366701332",
				Version: "3366701332",
				Addr:    "1.2.3.4",
			},
			Session: "-",
			Time:    "0 0",
			Addr:    "1.2.3.4",
			Media: []*sdp.Media{
				{
					Type:  sdp.MediaTypeAudio,
					Proto: "RTP/AVP",
					Port:  32898,
					Ptime: 20,
					Codecs: []*sdp.Codec{
						{PT: 9, Name: "G722", Rate: 8000},
						{PT: 18, Name: "G729", Rate: 8000},
						{PT: 0, Name: "PCMU", Rate: 8000},
						{PT: 101, Name: "telephone-event", Rate: 8000},
					},
				},
			},
		},
	},

	{
		name: "IPv6",
		s: "v=0\r\n" +
			"o=- 3366701332 3366701332 IN IP6 dead:beef::666\r\n" +
			"s=-\r\n" +
			"c=IN IP6 dead:beef::666\r\n" +
			"t=0 0\r\n" +
			"m=audio 32898 RTP/AVP 9 18 0 101\r\n" +
			"a=rtpmap:101 telephone-event/8000\r\n" +
			"a=ptime:20\r\n",
		s2: "v=0\r\n" +
			"o=- 3366701332 3366701332 IN IP6 dead:beef::666\r\n" +
			"s=-\r\n" +
			"c=IN IP6 dead:beef::666\r\n" +
			"t=0 0\r\n" +
			"m=audio 32898 RTP/AVP 9 18 0 101\r\n" +
			"a=rtpmap:9 G722/8000\r\n" +
			"a=rtpmap:18 G729/8000\r\n" +
			"a=rtpmap:0 PCMU/8000\r\n" +
			"a=rtpmap:101 telephone-event/8000\r\n" +
			"a=ptime:20\r\n",
		sdp: &sdp.SDP{
			Origin: &sdp.Origin{
				User:    "-",
				ID:      "3366701332",
				Version: "3366701332",
				Addr:    "dead:beef::666",
			},
			Session: "-",
			Time:    "0 0",
			Addr:    "dead:beef::666",
			Media: []*sdp.Media{
				{
					Type:  sdp.MediaTypeAudio,
					Proto: "RTP/AVP",
					Port:  32898,
					Ptime: 20,
					Codecs: []*sdp.Codec{
						{PT: 9, Name: "G722", Rate: 8000},
						{PT: 18, Name: "G729", Rate: 8000},
						{PT: 0, Name: "PCMU", Rate: 8000},
						{PT: 101, Name: "telephone-event", Rate: 8000},
					},
				},
			},
		},
	},

	{
		name: "pjmedia long sdp is long",
		s: ("v=0\r\n" +
			"o=- 3457169218 3457169218 IN IP4 10.11.34.37\r\n" +
			"s=pjmedia\r\n" +
			"c=IN IP4 10.11.34.37\r\n" +
			"t=0 0\r\n" +
			"m=audio 4000 RTP/AVP 103 102 104 113 3 0 8 9 101\r\n" +
			"a=rtpmap:103 speex/16000\r\n" +
			"a=rtpmap:102 speex/8000\r\n" +
			"a=rtpmap:104 speex/32000\r\n" +
			"a=rtpmap:113 iLBC/8000\r\n" +
			"a=fmtp:113 mode=30\r\n" +
			"a=rtpmap:3 GSM/8000\r\n" +
			"a=rtpmap:0 PCMU/8000\r\n" +
			"a=rtpmap:8 PCMA/8000\r\n" +
			"a=rtpmap:9 G722/8000\r\n" +
			"a=rtpmap:101 telephone-event/8000\r\n" +
			"a=fmtp:101 0-15\r\n" +
			"a=rtcp:4001 IN IP4 10.11.34.37\r\n" +
			"a=X-nat:0\r\n" +
			"a=ptime:20\r\n" +
			"a=sendrecv\r\n"),
		sdp: &sdp.SDP{
			Origin: &sdp.Origin{
				User:    "-",
				ID:      "3457169218",
				Version: "3457169218",
				Addr:    "10.11.34.37",
			},
			Session: "pjmedia",
			Time:    "0 0",
			Addr:    "10.11.34.37",
			Media: []*sdp.Media{
				{
					Type:      sdp.MediaTypeAudio,
					Proto:     "RTP/AVP",
					Port:      4000,
					Ptime:     20,
					Direction: sdp.SendRecv,
					Codecs: []*sdp.Codec{
						{PT: 103, Name: "speex", Rate: 16000},
						{PT: 102, Name: "speex", Rate: 8000},
						{PT: 104, Name: "speex", Rate: 32000},
						{PT: 113, Name: "iLBC", Rate: 8000, Fmtp: "mode=30"},
						{PT: 3, Name: "GSM", Rate: 8000},
						{PT: 0, Name: "PCMU", Rate: 8000},
						{PT: 8, Name: "PCMA", Rate: 8000},
						{PT: 9, Name: "G722", Rate: 8000},
						{PT: 101, Name: "telephone-event", Rate: 8000, Fmtp: "0-15"},
					},
					Attrs: [][2]string{
						{"rtcp", "4001 IN IP4 10.11.34.37"},
						{"X-nat", "0"},
					},
				},
			},
		},
	},

	{
		name: "mp3 tcp",
		s: ("v=0\r\n" +
			"o=- 3366701332 3366701334 IN IP4 10.11.34.37\r\n" +
			"s=squigglies\r\n" +
			"c=IN IP6 dead:beef::666\r\n" +
			"t=0 0\r\n" +
			"m=audio 80 TCP/IP 111\r\n" +
			"a=rtpmap:111 MP3/44100/2\r\n" +
			"a=sendonly\r\n"),
		sdp: &sdp.SDP{
			Origin: &sdp.Origin{
				User:    "-",
				ID:      "3366701332",
				Version: "3366701334",
				Addr:    "10.11.34.37",
			},
			Session: "squigglies",
			Time:    "0 0",
			Addr:    "dead:beef::666",
			Media: []*sdp.Media{
				{
					Type:  sdp.MediaTypeAudio,
					Proto: "TCP/IP",
					Port:  80,
					Codecs: []*sdp.Codec{
						{PT: 111, Name: "MP3", Rate: 44100, Param: "2"},
					},
					Direction: sdp.SendOnly,
				},
			},
		},
	},

	{
		name: "Kurento RTP",
		s: ("v=0\r\n" +
			"o=- 3896395953 3896395953 IN IP4 172.31.6.171\r\n" +
			"s=Kurento Media Server\r\n" +
			"c=IN IP4 172.31.6.171\r\n" +
			"t=0 0\r\n" +
			"m=audio 41094 RTP/AVPF 96 0 97\r\n" +
			"a=rtpmap:96 opus/48000/2\r\n" +
			"a=rtpmap:0 PCMU/8000\r\n" +
			"a=rtpmap:97 AMR/8000\r\n" +
			"m=video 51012 RTP/AVPF 102 103\r\n" +
			"a=rtpmap:102 VP8/90000\r\n" +
			"a=rtpmap:103 H264/90000\r\n" +
			"a=fmtp:103 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f\r\n" +
			"a=setup:actpass\r\n" +
			"a=extmap:3 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time\r\n" +
			"a=rtcp:41095\r\n" +
			"a=mid:audio0\r\n" +
			"a=ssrc:4148631681 cname:user1274683781@host-2b8db277\r\n" +
			"a=setup:actpass\r\n" +
			"a=extmap:3 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time\r\n" +
			"a=rtcp:51013\r\n" +
			"a=mid:video0\r\n" +
			"a=rtcp-fb:102 nack\r\n" +
			"a=rtcp-fb:102 nack pli\r\n" +
			"a=rtcp-fb:102 goog-remb\r\n" +
			"a=rtcp-fb:102 ccm fir\r\n" +
			"a=rtcp-fb:103 nack\r\n" +
			"a=rtcp-fb:103 nack pli\r\n" +
			"a=rtcp-fb:103 ccm fir\r\n" +
			"a=ssrc:1326927367 cname:user1274683781@host-2b8db277\r\n" +
			"a=sendrecv\r\n"),
		sdp: &sdp.SDP{
			Origin: &sdp.Origin{
				User:    "-",
				ID:      "3896395953",
				Version: "3896395953",
				Addr:    "172.31.6.171",
			},
			Session: "Kurento Media Server",
			Time:    "0 0",
			Addr:    "172.31.6.171",
			Media: []*sdp.Media{
				{
					Type:  sdp.MediaTypeAudio,
					Proto: "RTP/AVPF",
					Port:  41094,
					Codecs: []*sdp.Codec{
						{PT: 96, Name: "opus", Rate: 48000, Param: "2"},
						{PT: 0, Name: "PCMU", Rate: 8000},
						{PT: 97, Name: "AMR", Rate: 8000},
					},
				},
				{
					Type:      sdp.MediaTypeVideo,
					Proto:     "RTP/AVPF",
					Port:      51012,
					Direction: sdp.SendRecv,
					Codecs: []*sdp.Codec{
						{PT: 102, Name: "VP8", Rate: 90000},
						{PT: 103, Name: "H264", Rate: 90000, Fmtp: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f"},
					},
					Attrs: [][2]string{
						{"setup", "actpass"},
						{"extmap", "3 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time"},
						{"rtcp", "41095"},
						{"mid", "audio0"},
						{"ssrc", "4148631681 cname:user1274683781@host-2b8db277"},
						{"setup", "actpass"},
						{"extmap", "3 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time"},
						{"rtcp", "51013"},
						{"mid", "video0"},
						{"rtcp-fb", "102 nack"},
						{"rtcp-fb", "102 nack pli"},
						{"rtcp-fb", "102 goog-remb"},
						{"rtcp-fb", "102 ccm fir"},
						{"rtcp-fb", "103 nack"},
						{"rtcp-fb", "103 nack pli"},
						{"rtcp-fb", "103 ccm fir"},
						{"ssrc", "1326927367 cname:user1274683781@host-2b8db277"},
					},
				},
			},
		},
	},

	{
		name: "Kurento via Kamailio",
		s: ("v=0\r\n" +
			"o=- 3896394990 3896394990 IN IP4 192.0.2.10\r\n" +
			"s=Kurento Media Server\r\n" +
			"c=IN IP4 192.0.2.10\r\n" +
			"t=0 0\r\n" +
			"m=audio 50268 RTP/AVP 96 0\r\n" +
			"a=rtpmap:96 opus/48000/2\r\n" +
			"a=rtpmap:0 pcmu/8000\r\n" +
			"a=sendrecv\r\n" +
			"a=rtcp:50269\r\n" +
			"m=video 50302 RTP/AVP 102 103\r\n" +
			"a=ssrc:2163144404 cname:user539622331@host-6cf6de4c\r\n" +
			"a=rtcp-fb:102 nack\r\n" +
			"a=rtcp-fb:102 nack pli\r\n" +
			"a=rtcp-fb:102 goog-remb\r\n" +
			"a=rtcp-fb:102 ccm fir\r\n" +
			"a=rtcp-fb:103 nack\r\n" +
			"a=rtcp-fb:103 nack pli\r\n" +
			"a=rtcp-fb:103 ccm fir\r\n" +
			"a=ssrc:688187071 cname:user539622331@host-6cf6de4c\r\n" +
			"a=mid:audio0\r\n" +
			"a=rtpmap:102 VP8/90000\r\n" +
			"a=rtpmap:103 H264/90000\r\n" +
			"a=fmtp:103 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f\r\n" +
			"a=sendrecv\r\n" +
			"a=rtcp:50303\r\n"),
		s2: ("v=0\r\n" +
			"o=- 3896394990 3896394990 IN IP4 192.0.2.10\r\n" +
			"s=Kurento Media Server\r\n" +
			"c=IN IP4 192.0.2.10\r\n" +
			"t=0 0\r\n" +
			"m=audio 50268 RTP/AVP 96 0\r\n" +
			"a=rtpmap:96 opus/48000/2\r\n" +
			"a=rtpmap:0 pcmu/8000\r\n" +
			"a=rtcp:50269\r\n" +
			"a=sendrecv\r\n" +
			"m=video 50302 RTP/AVP 102 103\r\n" +
			"a=rtpmap:102 VP8/90000\r\n" +
			"a=rtpmap:103 H264/90000\r\n" +
			"a=fmtp:103 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f\r\n" +
			"a=ssrc:2163144404 cname:user539622331@host-6cf6de4c\r\n" +
			"a=rtcp-fb:102 nack\r\n" +
			"a=rtcp-fb:102 nack pli\r\n" +
			"a=rtcp-fb:102 goog-remb\r\n" +
			"a=rtcp-fb:102 ccm fir\r\n" +
			"a=rtcp-fb:103 nack\r\n" +
			"a=rtcp-fb:103 nack pli\r\n" +
			"a=rtcp-fb:103 ccm fir\r\n" +
			"a=ssrc:688187071 cname:user539622331@host-6cf6de4c\r\n" +
			"a=mid:audio0\r\n" +
			"a=rtcp:50303\r\n" +
			"a=sendrecv\r\n"),
		sdp: &sdp.SDP{
			Origin: &sdp.Origin{
				User:    "-",
				ID:      "3896394990",
				Version: "3896394990",
				Addr:    "192.0.2.10",
			},
			Session: "Kurento Media Server",
			Time:    "0 0",
			Addr:    "192.0.2.10",
			Media: []*sdp.Media{
				{
					Type:      sdp.MediaTypeAudio,
					Proto:     "RTP/AVP",
					Port:      50268,
					Direction: sdp.SendRecv,
					Codecs: []*sdp.Codec{
						{PT: 96, Name: "opus", Rate: 48000, Param: "2"},
						{PT: 0, Name: "pcmu", Rate: 8000},
					},
					Attrs: [][2]string{
						{"rtcp", "50269"},
					},
				},
				{
					Type:      sdp.MediaTypeVideo,
					Proto:     "RTP/AVP",
					Port:      50302,
					Direction: sdp.SendRecv,
					Codecs: []*sdp.Codec{
						{PT: 102, Name: "VP8", Rate: 90000},
						{PT: 103, Name: "H264", Rate: 90000, Fmtp: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f"},
					},
					Attrs: [][2]string{
						{"ssrc", "2163144404 cname:user539622331@host-6cf6de4c"},
						{"rtcp-fb", "102 nack"},
						{"rtcp-fb", "102 nack pli"},
						{"rtcp-fb", "102 goog-remb"},
						{"rtcp-fb", "102 ccm fir"},
						{"rtcp-fb", "103 nack"},
						{"rtcp-fb", "103 nack pli"},
						{"rtcp-fb", "103 ccm fir"},
						{"ssrc", "688187071 cname:user539622331@host-6cf6de4c"},
						{"mid", "audio0"},
						{"rtcp", "50303"},
					},
				},
			},
		},
	},

	{
		name: "Asterisk",
		s: ("v=0\r\n" +
			"o=- 3896394990 3896394992 IN IP4 192.0.2.200\r\n" +
			"s=Asterisk\r\n" +
			"c=IN IP4 192.0.2.200\r\n" +
			"t=0 0\r\n" +
			"m=audio 19540 RTP/AVP 0 96\r\n" +
			"a=rtpmap:0 PCMU/8000\r\n" +
			"a=rtpmap:96 opus/48000/2\r\n" +
			"a=ptime:20\r\n" +
			"a=maxptime:60\r\n" +
			"a=sendrecv\r\n" +
			"m=video 19252 RTP/AVP 103 102\r\n" +
			"a=rtpmap:103 H264/90000\r\n" +
			"a=fmtp:103 packetization-mode=1;level-asymmetry-allowed=1;profile-level-id=42E01F\r\n" +
			"a=rtpmap:102 VP8/90000\r\n" +
			"a=sendrecv\r\n"),
		sdp: &sdp.SDP{
			Origin: &sdp.Origin{
				User:    "-",
				ID:      "3896394990",
				Version: "3896394992",
				Addr:    "192.0.2.200",
			},
			Session: "Asterisk",
			Time:    "0 0",
			Addr:    "192.0.2.200",
			Media: []*sdp.Media{
				{
					Type:      sdp.MediaTypeAudio,
					Proto:     "RTP/AVP",
					Port:      19540,
					Ptime:     20,
					Maxptime:  60,
					Direction: sdp.SendRecv,
					Codecs: []*sdp.Codec{
						{PT: 0, Name: "PCMU", Rate: 8000},
						{PT: 96, Name: "opus", Rate: 48000, Param: "2"},
					},
				},
				{
					Type:      sdp.MediaTypeVideo,
					Proto:     "RTP/AVP",
					Port:      19252,
					Direction: sdp.SendRecv,
					Codecs: []*sdp.Codec{
						{PT: 103, Name: "H264", Rate: 90000, Fmtp: "packetization-mode=1;level-asymmetry-allowed=1;profile-level-id=42E01F"},
						{PT: 102, Name: "VP8", Rate: 90000},
					},
				},
			},
		},
	},

	{
		name: "Asterisk via Kamailio",
		s: ("v=0\r\n" +
			"o=- 3896394990 3896394992 IN IP4 192.0.2.11\r\n" +
			"s=Asterisk\r\n" +
			"c=IN IP4 192.0.2.11\r\n" +
			"t=0 0\r\n" +
			"m=audio 50286 RTP/AVPF 96\r\n" +
			"a=maxptime:60\r\n" +
			"a=rtpmap:96 opus/48000/2\r\n" +
			"a=sendrecv\r\n" +
			"a=rtcp:50287\r\n" +
			"a=ptime:20\r\n" +
			"m=video 50322 RTP/AVPF 103 102\r\n" +
			"a=mid:audio0\r\n" +
			"a=rtpmap:103 H264/90000\r\n" +
			"a=rtpmap:102 VP8/90000\r\n" +
			"a=fmtp:103 packetization-mode=1;level-asymmetry-allowed=1;profile-level-id=42E01F\r\n" +
			"a=sendrecv\r\n" +
			"a=rtcp:50323\r\n"),
		s2: ("v=0\r\n" +
			"o=- 3896394990 3896394992 IN IP4 192.0.2.11\r\n" +
			"s=Asterisk\r\n" +
			"c=IN IP4 192.0.2.11\r\n" +
			"t=0 0\r\n" +
			"m=audio 50286 RTP/AVPF 96\r\n" +
			"a=rtpmap:96 opus/48000/2\r\n" +
			"a=rtcp:50287\r\n" +
			"a=ptime:20\r\n" +
			"a=maxptime:60\r\n" +
			"a=sendrecv\r\n" +
			"m=video 50322 RTP/AVPF 103 102\r\n" +
			"a=rtpmap:103 H264/90000\r\n" +
			"a=fmtp:103 packetization-mode=1;level-asymmetry-allowed=1;profile-level-id=42E01F\r\n" +
			"a=rtpmap:102 VP8/90000\r\n" +
			"a=mid:audio0\r\n" +
			"a=rtcp:50323\r\n" +
			"a=sendrecv\r\n"),
		sdp: &sdp.SDP{
			Origin: &sdp.Origin{
				User:    "-",
				ID:      "3896394990",
				Version: "3896394992",
				Addr:    "192.0.2.11",
			},
			Session: "Asterisk",
			Time:    "0 0",
			Addr:    "192.0.2.11",
			Media: []*sdp.Media{
				{
					Type:      sdp.MediaTypeAudio,
					Proto:     "RTP/AVPF",
					Port:      50286,
					Ptime:     20,
					Maxptime:  60,
					Direction: sdp.SendRecv,
					Codecs: []*sdp.Codec{
						{PT: 96, Name: "opus", Rate: 48000, Param: "2"},
					},
					Attrs: [][2]string{
						{"rtcp", "50287"},
					},
				},
				{
					Type:      sdp.MediaTypeVideo,
					Proto:     "RTP/AVPF",
					Port:      50322,
					Direction: sdp.SendRecv,
					Codecs: []*sdp.Codec{
						{PT: 103, Name: "H264", Rate: 90000, Fmtp: "packetization-mode=1;level-asymmetry-allowed=1;profile-level-id=42E01F"},
						{PT: 102, Name: "VP8", Rate: 90000},
					},
					Attrs: [][2]string{
						{"mid", "audio0"},
						{"rtcp", "50323"},
					},
				},
			},
		},
	},

	{
		name: "Avaya no video support",
		s: ("v=0\r\n" +
			"o=- 1688577024 2 IN IP4 10.50.109.100\r\n" +
			"s=-\r\n" +
			"c=IN IP4 192.0.2.12\r\n" +
			"b=AS:64\r\n" +
			"t=0 0\r\n" +
			"m=audio 36568 RTP/AVP 0\r\n" +
			"c=IN IP4 192.0.2.12\r\n" +
			"a=sendrecv\r\n" +
			"a=ptime:20\r\n" +
			"m=video 0 RTP/AVP 103\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"a=inactive\r\n" +
			"a=rtpmap:103 H264/90000\r\n" +
			"a=ptime:20\r\n"),
		s2: ("v=0\r\n" +
			"o=- 1688577024 2 IN IP4 10.50.109.100\r\n" +
			"s=-\r\n" +
			"c=IN IP4 192.0.2.12\r\n" +
			"t=0 0\r\n" +
			"m=audio 36568 RTP/AVP 0\r\n" +
			"a=rtpmap:0 PCMU/8000\r\n" +
			"a=ptime:20\r\n" +
			"a=sendrecv\r\n"),
		sdp: &sdp.SDP{
			Origin: &sdp.Origin{
				User:    "-",
				ID:      "1688577024",
				Version: "2",
				Addr:    "10.50.109.100",
			},
			Session: "-",
			Time:    "0 0",
			Addr:    "192.0.2.12",
			Media: []*sdp.Media{
				{
					Type:      sdp.MediaTypeAudio,
					Proto:     "RTP/AVP",
					Port:      36568,
					Ptime:     20,
					Direction: sdp.SendRecv,
					Codecs: []*sdp.Codec{
						{PT: 0, Name: "PCMU", Rate: 8000},
					},
				},
			},
		},
	},
}

func sdpCompareCodec(t *testing.T, name string, correct, codec *sdp.Codec) {
	if correct != nil && codec == nil {
		t.Error(name, "not found")
	}
	if correct == nil && codec != nil {
		t.Error(name, "DO NOT WANT", codec)
	}
	if codec == nil {
		return
	}

	assert.Equal(t, correct.PT, codec.PT, "media %s - codec %d", name, correct.PT)
	assert.Equal(t, correct.Name, codec.Name, "media %s - codec %d", name, correct.PT)
	assert.Equal(t, correct.Rate, codec.Rate, "media %s - codec %d", name, correct.PT)
	assert.Equal(t, correct.Param, codec.Param, "media %s - codec %d", name, correct.PT)
	assert.Equal(t, correct.Fmtp, codec.Fmtp, "media %s - codec %d", name, correct.PT)
}

func sdpCompareCodecs(t *testing.T, name string, corrects, codecs []*sdp.Codec) {
	if corrects != nil && codecs == nil {
		t.Error(name, "codecs not found")
	}
	if corrects == nil && codecs != nil {
		t.Error(name, "codecs WUT", codecs)
	}
	if corrects == nil || codecs == nil {
		return
	}

	require.Len(t, codecs, len(corrects))

	for i := range corrects {
		c1, c2 := corrects[i], codecs[i]
		sdpCompareCodec(t, name, c1, c2)
	}
}

func sdpCompareMedia(t *testing.T, name string, correct, media *sdp.Media) {
	if correct != nil && media == nil {
		t.Error(name, "not found")
	}
	if correct == nil && media != nil {
		t.Error(name, "DO NOT WANT", media)
	}
	if correct == nil || media == nil {
		return
	}

	assert.Equal(t, correct.Type, media.Type, "media %s - %s - type", name, media.Type)
	assert.Equal(t, correct.Proto, media.Proto, "media %s - %s - proto", name, media.Type)
	assert.Equal(t, correct.Port, media.Port, "media %s - %s - port", name, media.Type)
	assert.Equal(t, correct.Ptime, media.Ptime, "media %s - %s - ptime", name, media.Type)
	assert.Equal(t, correct.Maxptime, media.Maxptime, "media %s - %s - maxptime", name, media.Type)

	if media.Codecs == nil || len(media.Codecs) < 1 {
		t.Error(name, "Must have at least one codec")
	}

	sdpCompareCodecs(t, name, correct.Codecs, media.Codecs)
}

func TestParse(t *testing.T) {
	for _, test := range sdpTests {
		sdp, err := sdp.Parse(test.s, false)
		if err != nil {
			if test.err == nil {
				t.Errorf("%v", err)
				continue
			} else { // test was supposed to fail
				panic("todo")
			}
		}

		if test.sdp.Addr != sdp.Addr {
			t.Error(test.name, "Addr", test.sdp.Addr, "!=", sdp.Addr)
		}
		if test.sdp.Origin.User != sdp.Origin.User {
			t.Error(test.name, "Origin.User", test.sdp.Origin.User, "!=",
				sdp.Origin.User)
		}
		if test.sdp.Origin.ID != sdp.Origin.ID {
			t.Error(test.name, "Origin.ID doesn't match")
		}
		if test.sdp.Origin.Version != sdp.Origin.Version {
			t.Error(test.name, "Origin.Version doesn't match")
		}
		if test.sdp.Origin.Addr != sdp.Origin.Addr {
			t.Error(test.name, "Origin.Addr doesn't match")
		}
		if test.sdp.Session != sdp.Session {
			t.Error(test.name, "Session", test.sdp.Session, "!=", sdp.Session)
		}
		if test.sdp.Time != sdp.Time {
			t.Error(test.name, "Time", test.sdp.Time, "!=", sdp.Time)
		}
		if test.sdp.Direction != sdp.Direction {
			t.Error(test.name, "Direction doesn't match: expected", test.sdp.Direction, "was", sdp.Direction)
		}

		if test.sdp.Attrs != nil {
			if sdp.Attrs == nil {
				t.Error(test.name, "Attrs weren't extracted")
			} else if len(sdp.Attrs) != len(test.sdp.Attrs) {
				t.Error(test.name, "Attrs length not same")
			} else {
				for i := range sdp.Attrs {
					p1, p2 := test.sdp.Attrs[i], sdp.Attrs[i]
					if p1[0] != p2[0] || p1[1] != p2[1] {
						t.Error(test.name, "attr", p1, "!=", p2)
						break
					}
				}
			}
		} else {
			if sdp.Attrs != nil {
				t.Error(test.name, "unexpected attrs", sdp.Attrs)
			}
		}

		if len(test.sdp.Media) != len(sdp.Media) {
			t.Errorf("%s: incorrect media count: expected %d, was %d", test.name, len(test.sdp.Media), len(sdp.Media))
		} else {
			for n := range test.sdp.Media {
				sdpCompareMedia(t, test.name, test.sdp.Media[n], sdp.Media[n])
			}
		}
	}
}

func TestFormatSDP(t *testing.T) {
	for _, test := range sdpTests {
		sdp := test.sdp.String()
		s := test.s
		if test.s2 != "" {
			s = test.s2
		}
		if s != sdp {
			t.Errorf("\nTest: %s\n\nExpected:\n%+v\n\nFound:\n%+v\n\n", test.name, s, sdp)
			fmt.Printf("%s", sdp)
		}
	}
}
