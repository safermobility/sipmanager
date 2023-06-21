# SIP Manager

This project acts as a SIP UAC to manage calls for an application.
It handles only signalling, not media.

It is heavily inspired by the `dialog` package of [jart/gosip](https://github.com/jart/gosip), but that package can only handle a single call at a time.
It also draws some inspiration from [JAIN SIP API](https://www.oracle.com/technical-resources/articles/enterprise-architecture/introduction-jain-sip-part1.html) (`javax.sip` package), and the [NIST Reference Implementation](https://github.com/usnistgov/jsip) of that API.

Support is also added for passing all traffic to an Outbound Proxy server instead of directly to the destination.

This package has been tested with [Kurento](https://kurento.openvidu.io/) (`RtpEndpoint`) providing the media, and [Kamailio](https://www.kamailio.org/) as a proxy.

We would like to use the SIP and SDP parsers from `gosip` directly, but we cannot import `gosip` directly because the `dsp` package has assembly code that will not compile on ARM processors. Instead, large parts of those two packages (and the `util` package) are copied here.
