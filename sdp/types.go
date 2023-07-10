package sdp

type MediaDirection string

const (
	SendRecv MediaDirection = "sendrecv"
	SendOnly MediaDirection = "sendonly"
	RecvOnly MediaDirection = "recvonly"
	Inactive MediaDirection = "inactive"
)

// Known media types from RFC8866 sections 5.14 and 8.2.2, and other places
type MediaType string

const (
	MediaTypeAudio       MediaType = "audio"
	MediaTypeVideo       MediaType = "video"
	MediaTypeText        MediaType = "text"
	MediaTypeApplication MediaType = "application"
	MediaTypeMessage     MediaType = "message"
	MediaTypeImage       MediaType = "image" // RFC6466
)

func IsKnownMediaType(name string) (MediaType, bool) {
	switch MediaType(name) {
	case MediaTypeAudio,
		MediaTypeVideo,
		MediaTypeText,
		MediaTypeApplication,
		MediaTypeMessage,
		MediaTypeImage:
		return MediaType(name), true
	default:
		return "", false
	}
}

type TransportProtocol string

const (
	ProtoRTPAVP   TransportProtocol = "RTP/AVP"
	ProtoRTPAVPF  TransportProtocol = "RTP/AVPF"
	ProtoRTPSAVP  TransportProtocol = "RTP/SAVP"
	ProtoRTPSAVPF TransportProtocol = "RTP/SAVPF"
	ProtocolTCPIP TransportProtocol = "TCP/IP"
	ProtocolUDP   TransportProtocol = "udp"
)

func IsKnownTransportProtocol(name string) (TransportProtocol, bool) {
	switch TransportProtocol(name) {
	case ProtoRTPAVP,
		ProtoRTPAVPF,
		ProtoRTPSAVP,
		ProtoRTPSAVPF,
		ProtocolTCPIP,
		ProtocolUDP:
		return TransportProtocol(name), true
	default:
		return "", false
	}
}
