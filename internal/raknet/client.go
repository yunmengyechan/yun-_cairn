package raknet

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type HandshakeInfo struct {
	ServerGUID  uint64
	Cookie      uint32
	HasSecurity bool
	MTU         int
	Reply1      []byte
	Reply2      []byte
	Accepted    []byte
}

type Client struct {
	conn       *net.UDPConn
	guid       uint64
	mtu        int
	serverIP   [4]byte
	serverPort int

	mu   sync.Mutex
	info HandshakeInfo

	online  bool
	running atomic.Bool
	closed  atomic.Bool

	sendSeq     atomic.Uint32
	reliableIdx atomic.Uint32
	orderedIdx  atomic.Uint32

	sentPkts  atomic.Uint64
	recvPkts  atomic.Uint64
	sentBytes atomic.Uint64
	recvBytes atomic.Uint64

	ackMu      sync.Mutex
	ackPending []uint32
	nakPending []uint32

	sendQ chan *Frame
	ctrlQ chan []byte
	gameQ chan []byte

	splitter  *Splitter
	assembler *Assembler

	OnLog     func(string)
	OnMessage func([]byte)
}

func New() *Client {
	return &Client{
		guid:      uint64(time.Now().UnixNano()) ^ 0x9e3779b97f4a7c15,
		mtu:       DEFAULT_MTU,
		sendQ:     make(chan *Frame, 4096),
		ctrlQ:     make(chan []byte, 256),
		gameQ:     make(chan []byte, 4096),
		splitter:  NewSplitter(),
		assembler: NewAssembler(),
	}
}

func (c *Client) dbg(f string, a ...any) {
	if os.Getenv("PH_DEBUG") != "" {
		c.log(f, a...)
	}
}

func (c *Client) log(f string, a ...any) {
	if c.OnLog != nil {
		c.OnLog(fmt.Sprintf(f, a...))
	}
}

func (c *Client) logHex(tag string, b []byte) {
	n := len(b)
	if n > 48 {
		n = 48
	}
	c.log("%s %dB  %x", tag, len(b), b[:n])
}

func (c *Client) Handshake(host string, port int, timeout time.Duration) (*HandshakeInfo, error) {
	raddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp4", nil, raddr)
	if err != nil {
		return nil, err
	}
	c.conn = conn
	c.serverPort = port
	copy(c.serverIP[:], raddr.IP.To4())

	conn.SetReadBuffer(8 * 1024 * 1024)
	conn.SetWriteBuffer(8 * 1024 * 1024)

	deadline := time.Now().Add(timeout)

	req1 := []byte{IDOpenConnectionRequest1}
	req1 = append(req1, Magic...)
	req1 = append(req1, RAKNET_PROTOCOL_VERSION)
	pad := c.mtu - 28 - len(req1)
	if pad > 0 {
		req1 = append(req1, make([]byte, pad)...)
	}
	if err := c.sendRaw(req1); err != nil {
		return nil, err
	}
	c.logHex("TX OCR1", req1)

	reply1, err := c.recvPred(deadline, 5*time.Second, func(b []byte) bool {
		return len(b) > 0 && (b[0] == IDOpenConnectionReply1 || b[0] == IDIncompatibleProtocol)
	})
	if err != nil {
		return nil, fmt.Errorf("等 Reply1 失败: %w", err)
	}
	c.logHex("RX", reply1)
	if reply1[0] == IDIncompatibleProtocol {
		if len(reply1) >= 2 {
			return nil, fmt.Errorf("服务器要协议版本 %d（我发 %d）", reply1[1], RAKNET_PROTOCOL_VERSION)
		}
		return nil, errors.New("协议不兼容")
	}
	if len(reply1) < 28 {
		return nil, fmt.Errorf("Reply1 太短: %d", len(reply1))
	}

	off := 17
	c.info.ServerGUID = binary.BigEndian.Uint64(reply1[off : off+8])
	off += 8

	if len(reply1) >= 32 {
		sec := reply1[off]
		off++
		c.info.HasSecurity = sec != 0
		if c.info.HasSecurity {
			c.info.Cookie = binary.BigEndian.Uint32(reply1[off : off+4])
			off += 4
		}
		c.info.MTU = int(binary.BigEndian.Uint16(reply1[off : off+2]))
		if c.info.MTU < 500 || c.info.MTU > 2000 {
			c.info.MTU = 1400
		}
		c.mtu = c.info.MTU
		c.log("Reply1: guid=%016x sec=%d cookie=%08x mtu=%d",
			c.info.ServerGUID, sec, c.info.Cookie, c.info.MTU)
	} else {
		c.info.MTU = int(binary.BigEndian.Uint16(reply1[off : off+2]))
		if c.info.MTU < 500 || c.info.MTU > 2000 {
			c.info.MTU = 1400
		}
		c.mtu = c.info.MTU
		c.info.HasSecurity = false
		c.log("Reply1: guid=%016x mtu=%d (标准格式)", c.info.ServerGUID, c.info.MTU)
	}
	c.info.Reply1 = append([]byte(nil), reply1...)

	req2 := []byte{IDOpenConnectionRequest2}
	req2 = append(req2, Magic...)
	if c.info.HasSecurity {
		var ck [4]byte
		binary.BigEndian.PutUint32(ck[:], c.info.Cookie)
		req2 = append(req2, ck[:]...)
	}
	req2 = append(req2, WriteAddress(c.serverIP, c.serverPort)...)
	var mb [2]byte
	binary.BigEndian.PutUint16(mb[:], uint16(c.mtu))
	req2 = append(req2, mb[:]...)
	var gb [8]byte
	binary.BigEndian.PutUint64(gb[:], c.guid)
	req2 = append(req2, gb[:]...)
	if err := c.sendRaw(req2); err != nil {
		return nil, err
	}
	c.logHex("TX OCR2", req2)

	reply2, err := c.recvPred(deadline, 5*time.Second, func(b []byte) bool {
		return len(b) > 0 && b[0] == IDOpenConnectionReply2
	})
	if err != nil {
		return nil, fmt.Errorf("等 Reply2 失败: %w", err)
	}
	c.logHex("RX", reply2)
	c.info.Reply2 = append([]byte(nil), reply2...)
	c.log("Reply2 已收")

	c.running.Store(true)
	go c.recvLoop()
	go c.sendLoop()
	go c.ackLoop()

	body := []byte{IDConnectionRequest}
	var g2 [8]byte
	binary.BigEndian.PutUint64(g2[:], c.guid)
	body = append(body, g2[:]...)
	var t2 [8]byte
	binary.BigEndian.PutUint64(t2[:], uint64(time.Now().UnixMilli()))
	body = append(body, t2[:]...)
	body = append(body, 0)
	c.log("TX ConnectionRequest(帧) %dB  %x", len(body), body)
	c.queueFrame(&Frame{Reliability: RelReliableOrdered, Body: body})

	waitEnd := time.Now().Add(8 * time.Second)
	lastResend := time.Now()
	for time.Now().Before(waitEnd) {
		if time.Since(lastResend) > 1200*time.Millisecond {
			lastResend = time.Now()
			c.log("重发 ConnectionRequest")
			c.queueFrame(&Frame{Reliability: RelReliableOrdered, Body: body})
		}
		select {
		case pkt := <-c.ctrlQ:
			if len(pkt) > 0 {
				c.log("CTRL %dB id=0x%02x %x", len(pkt), pkt[0], pkt)
			}
			if len(pkt) > 0 && pkt[0] == IDConnectionRequestAccepted {
				c.logHex("RX Accepted", pkt)
				c.info.Accepted = append([]byte(nil), pkt...)
				c.sendNewIncoming(pkt)
				c.sendConnectedPing()
				c.online = true
				return &c.info, nil
			}
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	c.log("ConnectionRequestAccepted 未到")
	c.online = true
	return &c.info, nil
}

func (c *Client) sendNewIncoming(cra []byte) {
	body := []byte{IDNewIncomingConnection}
	body = append(body, WriteAddress(c.serverIP, c.serverPort)...)

	blackhole := [4]byte{255, 255, 255, 255}
	for i := 0; i < 20; i++ {
		body = append(body, WriteAddress(blackhole, 0)...)
	}
	var t1 [8]byte
	binary.BigEndian.PutUint64(t1[:], uint64(time.Now().UnixMilli()))
	body = append(body, t1[:]...)
	body = append(body, t1[:]...)
	c.log("TX NewIncomingConnection %dB", len(body))
	c.queueFrame(&Frame{Reliability: RelReliableOrdered, Body: body})
}

func (c *Client) sendConnectedPing() {
	body := []byte{IDConnectedPing}
	var t [8]byte
	binary.BigEndian.PutUint64(t[:], uint64(time.Now().UnixMilli()))
	body = append(body, t[:]...)
	c.queueFrame(&Frame{Reliability: RelUnreliable, Body: body})
}

func (c *Client) SendGame(payload []byte) error {
	if !c.online {
		return errors.New("未连接")
	}
	full := append([]byte{IDGamePacket}, payload...)

	maxBody := c.mtu - 60
	if maxBody < 256 {
		maxBody = 256
	}
	rel := RelUnreliable
	if len(full) > maxBody {
		rel = RelReliable
	}
	c.queueFrame(&Frame{Reliability: rel, Body: full})
	return nil
}

func (c *Client) queueFrame(f *Frame) {
	select {
	case c.sendQ <- f:
	default:
	}
}

func (c *Client) sendLoop() {
	for c.running.Load() {
		select {
		case f := <-c.sendQ:

			frags := c.splitter.Split(f.Body, f.Reliability)
			if len(frags) > 1 {
				c.log("分片：%d 字节 -> %d 片", len(f.Body), len(frags))
			}
			for _, fr := range frags {
				if IsReliable(fr.Reliability) {
					fr.ReliableIndex = c.reliableIdx.Add(1) - 1
				}
				if IsOrdered(fr.Reliability) {
					fr.OrderedIndex = c.orderedIdx.Add(1) - 1
				}
				sq := c.sendSeq.Add(1) - 1
				dg := PackFrameSet(sq, []*Frame{fr})
				lim := len(dg)
				if lim > 72 {
					lim = 72
				}
				c.dbg("TXFRAME seq=%d rel=%d frag=%v %dB  %x",
					sq, fr.ReliableIndex, fr.Fragmented, len(dg), dg[:lim])
				c.sendRaw(dg)
			}
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func (c *Client) ackLoop() {
	for c.running.Load() {
		time.Sleep(10 * time.Millisecond)
		c.ackMu.Lock()
		if len(c.ackPending) == 0 && len(c.nakPending) == 0 {
			c.ackMu.Unlock()
			continue
		}
		acks := c.ackPending
		naks := c.nakPending
		c.ackPending = nil
		c.nakPending = nil
		c.ackMu.Unlock()
		if len(acks) > 0 {
			c.sendRaw(packAckNak(IDAck, acks))
		}
		if len(naks) > 0 {
			c.sendRaw(packAckNak(IDNack, naks))
		}
	}
}

func (c *Client) recvLoop() {
	buf := make([]byte, 65536)
	for c.running.Load() {
		c.conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, err := c.conn.Read(buf)
		if err != nil {
			continue
		}
		if n <= 0 {
			continue
		}
		c.recvPkts.Add(1)
		c.recvBytes.Add(uint64(n))
		c.handleDatagram(buf[:n])
	}
}

func packAckNak(kind byte, seqs []uint32) []byte {
	b := []byte{kind}
	var cnt [2]byte
	binary.BigEndian.PutUint16(cnt[:], 1)
	b = append(b, cnt[:]...)
	b = append(b, 1)
	b = appendUint24LE(b, seqs[len(seqs)-1])
	return b
}

func (c *Client) handleDatagram(data []byte) {
	if len(data) == 0 {
		return
	}
	id := data[0]

	{
		lim := len(data)
		if lim > 64 {
			lim = 64
		}
		c.dbg("RXDAT %dB  id=0x%02x  %x", len(data), id, data[:lim])
	}
	if id == IDAck || id == IDNack {
		return
	}
	if id >= 0x80 && id <= 0x8d {
		_, frames, err := UnpackFrameSet(data)
		if err != nil {
			return
		}
		for _, f := range frames {

			if IsReliable(f.Reliability) {
				c.ackMu.Lock()
				c.ackPending = append(c.ackPending, f.ReliableIndex)
				c.ackMu.Unlock()
			}

			body := f.Body
			if f.Fragmented {
				merged := c.assembler.Feed(f)
				if merged == nil {
					continue
				}
				body = merged
			}
			if len(body) == 0 {
				continue
			}
			pid := body[0]
			switch {
			case pid == IDGamePacket:
				select {
				case c.gameQ <- body[1:]:
				default:
				}
			case pid == IDConnectedPing:
				c.sendConnectedPong(body)
			default:
				select {
				case c.ctrlQ <- body:
				default:
				}
			}
		}
	}
}

func (c *Client) sendConnectedPong(ping []byte) {
	body := []byte{IDConnectedPong}
	if len(ping) >= 9 {
		body = append(body, ping[1:9]...)
	} else {
		var z [8]byte
		body = append(body, z[:]...)
	}
	var t [8]byte
	binary.BigEndian.PutUint64(t[:], uint64(time.Now().UnixMilli()))
	body = append(body, t[:]...)
	c.queueFrame(&Frame{Reliability: RelUnreliable, Body: body})
}

func (c *Client) sendRaw(b []byte) error {
	if c.conn == nil {
		return errors.New("未连接")
	}
	n, err := c.conn.Write(b)
	if n > 0 {
		c.sentPkts.Add(1)
		c.sentBytes.Add(uint64(n))
	}
	return err
}

func (c *Client) recvPred(deadline time.Time, window time.Duration, pred func([]byte) bool) ([]byte, error) {
	end := time.Now().Add(window)
	if end.After(deadline) {
		end = deadline
	}
	buf := make([]byte, 4096)
	for time.Now().Before(end) {
		c.conn.SetReadDeadline(end)
		n, err := c.conn.Read(buf)
		if err != nil {
			return nil, err
		}
		if n <= 0 {
			continue
		}
		c.recvPkts.Add(1)
		c.recvBytes.Add(uint64(n))
		b := append([]byte(nil), buf[:n]...)
		if pred(b) {
			return b, nil
		}
	}
	return nil, errors.New("超时")
}

func (c *Client) RecvGame(timeout time.Duration) ([]byte, error) {
	select {
	case b := <-c.gameQ:
		return b, nil
	case <-time.After(timeout):
		return nil, errors.New("超时")
	}
}

func (c *Client) Stats() (sent, recv, sentB, recvB uint64) {
	return c.sentPkts.Load(), c.recvPkts.Load(), c.sentBytes.Load(), c.recvBytes.Load()
}

func (c *Client) Close() {
	c.running.Store(false)
	time.Sleep(50 * time.Millisecond)
	if c.conn != nil {
		c.conn.Close()
	}
}
