// --- START OF COMPLETE REPLACEMENT for liuproxy-remote/internal/agent/socks5/helpers.go ---
package tunnel

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
)

// parseMetadata 从解密后的数据中解析元数据
func (a *Agent) parseMetadata(data []byte) (byte, string, error) {
	if len(data) < 2 {
		return 0, "", fmt.Errorf("metadata packet too short")
	}
	cmd := data[0]
	addrType := data[1]
	var host string
	var port int
	offset := 2

	switch addrType {
	case 0x01: // IPv4
		if len(data) < offset+4+2 {
			return 0, "", fmt.Errorf("invalid ipv4 metadata")
		}
		host = net.IP(data[offset : offset+4]).String()
		offset += 4
	case 0x03: // Domain
		domainLen := int(data[offset])
		offset++
		if len(data) < offset+domainLen+2 {
			return 0, "", fmt.Errorf("invalid domain metadata")
		}
		host = string(data[offset : offset+domainLen])
		offset += domainLen
	case 0x04: // IPv6
		if len(data) < offset+16+2 {
			return 0, "", fmt.Errorf("invalid ipv6 metadata")
		}
		host = net.IP(data[offset : offset+16]).String()
		offset += 16
	default:
		return 0, "", fmt.Errorf("unsupported address type in metadata: %d", addrType)
	}

	port = int(binary.BigEndian.Uint16(data[offset : offset+2]))
	return cmd, net.JoinHostPort(host, strconv.Itoa(port)), nil
}
