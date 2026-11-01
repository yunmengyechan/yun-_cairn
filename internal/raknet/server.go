package raknet

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"time"
)

type ServerSide struct {
	Conn       *net.UDPConn
	ClientAddr *net.UDPAddr
	GUID       uint64
	MTU        int
	Cookie     uint32

	SendSeq uint32
	RecvSeq uint32
	Log     func(string)
}

func (s *ServerSide) logf(f string, a ...any) {
	if s.Log != nil {
		s.Log(fmt.Sprintf(f, a...))
	}
}

func NewServerSide(port int) (*ServerSide, error) {
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return nil, err
	}
	conn.SetReadBuffer(8 * 1024 * 1024)
	conn.SetWriteBuffer(8 * 1024 * 1024)
	return &ServerSide{
		Conn: conn,
		GUID: rand.Uint64(),
		MTU:  1400,
	}, nil
}

func (s *ServerSide) Accept(timeout time.Duration) (*net.UDPAddr, error) {
	deadline := time.Now().Add(timeout)
	buf := make([]byte, 2048)

	step := ""
	for time.Now().Before(deadline) {
		s.Conn.SetReadDeadline(deadline)
		n, from, err := s.Conn.ReadFromUDP(buf)
		if err != nil {
			return nil, err
		}
		pkt := buf[:n]
		if len(pkt) == 0 {
			continue
		}
		id := pkt[0]

		switch id {
		case IDOpenConnectionRequest1:

			clientMTU := len(pkt) + 28
			if clientMTU < 576 {
				clientMTU = 576
			}
			if clientMTU > 1492 {
				clientMTU = 1492
			}
			s.MTU = clientMTU
			s.ClientAddr = from
			s.Cookie = rand.Uint32()
			s.logf("RX OCR1  %dB  clientMTU=%d", len(pkt), clientMTU)

			reply := []byte{IDOpenConnectionReply1}
			reply = append(reply, Magic...)
			var g [8]byte
			binary.BigEndian.PutUint64(g[:], s.GUID)
			reply = append(reply, g[:]...)
			reply = append(reply, 1)
			var ck [4]byte
			binary.BigEndian.PutUint32(ck[:], s.Cookie)
			reply = append(reply, ck[:]...)
			var mb [2]byte
			binary.BigEndian.PutUint16(mb[:], uint16(s.MTU))
			reply = append(reply, mb[:]...)
			s.Conn.WriteToUDP(reply, from)
			s.logf("TX Reply1  guid=%016x cookie=%08x mtu=%d", s.GUID, s.Cookie, s.MTU)
			step = "reply1"

		case IDOpenConnectionRequest2:

			s.logf("RX OCR2  %dB", len(pkt))

			reply := []byte{IDOpenConnectionReply2}
			reply = append(reply, Magic...)
			var g2 [8]byte
			binary.BigEndian.PutUint64(g2[:], s.GUID)
			reply = append(reply, g2[:]...)
			reply = append(reply, WriteAddress([4]byte{from.IP[0], from.IP[1], from.IP[2], from.IP[3]}, from.Port)...)
			var mb2 [2]byte
			binary.BigEndian.PutUint16(mb2[:], uint16(s.MTU))
			reply = append(reply, mb2[:]...)
			reply = append(reply, 0)
			s.Conn.WriteToUDP(reply, from)
			s.logf("TX Reply2  %dB", len(reply))
			step = "reply2"

		case IDAck, IDNack:

		default:

			if pkt[0]&0x80 != 0 {

				seq, frames, err := UnpackFrameSet(pkt)
				if err != nil {
					continue
				}
				_ = seq
				for _, f := range frames {
					if len(f.Body) == 0 {
						continue
					}
					switch f.Body[0] {
					case IDConnectionRequest:
						s.logf("RX ConnectionRequest  %dB", len(f.Body))

						accept := s.buildAccepted(from)
						s.sendFrame(accept, RelReliableOrdered)
						s.logf("TX ConnectionRequestAccepted  %dB", len(accept))
						step = "accepted"
					case IDNewIncomingConnection:
						s.logf("RX NewIncomingConnection  %dB", len(f.Body))
						s.logf("✓ RakNet 握手完成  客户端=%s  mtu=%d", from.String(), s.MTU)
						return from, nil
					case IDConnectedPing:

						if len(f.Body) >= 9 {
							pong := append([]byte{IDConnectedPong}, f.Body[1:9]...)
							pong = append(pong, f.Body[1:9]...)
							s.sendFrame(pong, RelUnreliable)
						}
					}
				}
			}
		}
	}
	return nil, fmt.Errorf("Accept 超时（最后一步: %s）", step)
}

func (s *ServerSide) buildAccepted(client *net.UDPAddr) []byte {
	b := []byte{IDConnectionRequestAccepted}
	b = append(b, WriteAddress([4]byte{client.IP[0], client.IP[1], client.IP[2], client.IP[3]}, client.Port)...)
	var sys [2]byte
	binary.BigEndian.PutUint16(sys[:], 0)
	b = append(b, sys[:]...)

	for i := 0; i < 20; i++ {
		b = append(b, 0x04, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff)
	}
	now := uint64(time.Now().UnixMilli())
	var t8 [8]byte
	binary.BigEndian.PutUint64(t8[:], now)
	b = append(b, t8[:]...)
	b = append(b, t8[:]...)
	return b
}

func (s *ServerSide) sendFrame(data []byte, reliability int) {
	s.SendSeq++
	f := &Frame{Body: data, Reliability: reliability}
	pkt := PackFrameSet(s.SendSeq, []*Frame{f})
	s.Conn.WriteToUDP(pkt, s.ClientAddr)
}

func (s *ServerSide) SendGame(payload []byte) error {
	s.sendFrame(payload, RelReliableOrdered)
	return nil
}

func (s *ServerSide) RecvGame(timeout time.Duration) ([]byte, error) {
	deadline := time.Now().Add(timeout)
	buf := make([]byte, 4096)
	for time.Now().Before(deadline) {
		s.Conn.SetReadDeadline(deadline)
		n, from, err := s.Conn.ReadFromUDP(buf)
		if err != nil {
			return nil, err
		}
		if from.String() != s.ClientAddr.String() {
			continue
		}
		pkt := buf[:n]
		if len(pkt) == 0 || pkt[0]&0x80 == 0 {
			continue
		}
		_, frames, err := UnpackFrameSet(pkt)
		if err != nil {
			continue
		}
		for _, f := range frames {
			if len(f.Body) > 0 {
				return f.Body, nil
			}
		}
	}
	return nil, fmt.Errorf("RecvGame 超时")
}

func (s *ServerSide) Close() {
	if s.Conn != nil {
		s.Conn.Close()
	}
}
