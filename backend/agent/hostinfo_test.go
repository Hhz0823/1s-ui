package agent

import "testing"

func TestParseSockstatText(t *testing.T) {
	v4 := "sockets: used 50\nTCP: inuse 38 orphan 0 tw 2 alloc 38 mem 93\nUDP: inuse 4 mem 0\nUDPLITE: inuse 0\n"
	v6 := "TCP6: inuse 3\nUDP6: inuse 1\nUDPLITE6: inuse 0\n"
	tcp, udp := parseSockstatText(v4 + v6)
	if tcp != 41 || udp != 5 {
		t.Fatalf("tcp/udp = %d/%d, want 41/5", tcp, udp)
	}
}
