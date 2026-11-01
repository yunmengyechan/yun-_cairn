package raknet

import "encoding/binary"

var Magic = []byte{
	0x00, 0xff, 0xff, 0x00, 0xfe, 0xfe, 0xfe, 0xfe,
	0xfd, 0xfd, 0xfd, 0xfd, 0x12, 0x34, 0x56, 0x78,
}

const RAKNET_PROTOCOL_VERSION = 8

const DEFAULT_MTU = 1492

const (
	IDUnconnectedPing        = 0x01
	IDUnconnectedPong        = 0x1c
	IDOpenConnectionRequest1 = 0x05
	IDOpenConnectionReply1   = 0x06
	IDOpenConnectionRequest2 = 0x07
	IDOpenConnectionReply2   = 0x08
	IDIncompatibleProtocol   = 0x19
)

const (
	IDConnectedPing             = 0x00
	IDConnectedPong             = 0x03
	IDDetectLostConnections     = 0x04
	IDConnectionRequest         = 0x09
	IDConnectionRequestAccepted = 0x10
	IDAlreadyConnected          = 0x12
	IDNewIncomingConnection     = 0x13
	IDConnectionAttemptFailed   = 0x14
	IDDisconnectionNotification = 0x15
	IDConnectionLost            = 0x16
)

const (
	IDAck  = 0xc0
	IDNack = 0xa0
)

const IDGamePacket = 0xfe

const (
	RelUnreliable          = 0
	RelUnreliableSequenced = 1
	RelReliable            = 2
	RelReliableOrdered     = 3
	RelReliableSequenced   = 4
	RelUnreliableAck       = 5
	RelReliableOrderedAck  = 7
)

func IsReliable(r int) bool {
	return r == RelReliable || r == RelReliableOrdered ||
		r == RelReliableSequenced || r == RelReliableOrderedAck
}

func IsSequenced(r int) bool {
	return r == RelUnreliableSequenced || r == RelReliableSequenced
}

func IsOrdered(r int) bool {
	return r == RelReliableOrdered || r == RelReliableOrderedAck
}

func writeUint24LE(v uint32) []byte {
	return []byte{byte(v), byte(v >> 8), byte(v >> 16)}
}

func readUint24LE(b []byte, off int) (uint32, int) {
	v := uint32(b[off]) | uint32(b[off+1])<<8 | uint32(b[off+2])<<16
	return v, off + 3
}

func appendUint24LE(b []byte, v uint32) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16))
}

func WriteAddress(ip [4]byte, port int) []byte {
	b := make([]byte, 0, 7)
	b = append(b, 4)
	b = append(b, ip[:]...)
	var p [2]byte
	binary.BigEndian.PutUint16(p[:], uint16(port))
	b = append(b, p[:]...)
	return b
}

func ReadAddress(b []byte, off int) (string, int, int) {
	if off >= len(b) {
		return "", 0, 0
	}
	ver := b[off]
	off++
	if ver == 4 {
		if off+6 > len(b) {
			return "", 0, 0
		}
		ip := itoa(b[off]) + "." + itoa(b[off+1]) + "." + itoa(b[off+2]) + "." + itoa(b[off+3])
		port := int(binary.BigEndian.Uint16(b[off+4 : off+6]))
		return ip, port, 7
	}
	if ver == 6 {

		return "", 0, 0
	}
	return "", 0, 0
}

func itoa(v byte) string {
	if v == 0 {
		return "0"
	}
	var buf [3]byte
	i := 0
	for v > 0 {
		buf[i] = byte(0x30 + v%10)
		v /= 10
		i++
	}

	out := make([]byte, i)
	for j := 0; j < i; j++ {
		out[j] = buf[i-1-j]
	}
	return string(out)
}
