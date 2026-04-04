//go:build linux

package agent

import (
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// runICMP performs a real ICMP echo probe on Linux.
// It requires CAP_NET_RAW or root privilege.
func runICMP(target string) (latency float64, success bool) {
	ipAddr, err := net.ResolveIPAddr("ip4", target)
	if err != nil {
		return 0, false
	}

	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return 0, false
	}
	defer conn.Close()

	deadline := time.Now().Add(5 * time.Second)
	_ = conn.SetDeadline(deadline)

	echoID := os.Getpid() & 0xffff
	echoSeq := int(time.Now().UnixNano() & 0xffff)
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   echoID,
			Seq:  echoSeq,
			Data: []byte("cloudnest-icmp"),
		},
	}
	payload, err := msg.Marshal(nil)
	if err != nil {
		return 0, false
	}

	start := time.Now()
	if _, err := conn.WriteTo(payload, ipAddr); err != nil {
		return 0, false
	}

	reply := make([]byte, 1500)
	for {
		n, peer, err := conn.ReadFrom(reply)
		if err != nil {
			return 0, false
		}
		peerIPAddr, ok := peer.(*net.IPAddr)
		if !ok || !peerIPAddr.IP.Equal(ipAddr.IP) {
			continue
		}

		parsed, err := icmp.ParseMessage(1, reply[:n]) // 1 = ICMP for IPv4
		if err != nil {
			continue
		}
		if parsed.Type != ipv4.ICMPTypeEchoReply {
			continue
		}
		body, ok := parsed.Body.(*icmp.Echo)
		if !ok || body.ID != echoID || body.Seq != echoSeq {
			continue
		}
		return float64(time.Since(start).Milliseconds()), true
	}
}
