package mc

import (
	"fmt"
	"strconv"
	"strings"

	"cairn/internal/raknet"
)

type addrCand struct {
	host string
	port int
	tag  string
}

func (a addrCand) label() string {
	if a.tag == "" {
		return fmt.Sprintf("%s:%d", a.host, a.port)
	}
	return fmt.Sprintf("%s:%d(%s)", a.host, a.port, a.tag)
}

func (g *GameServer) Candidates() []addrCand {
	if g == nil {
		return nil
	}
	var out []addrCand
	seen := map[string]bool{}
	add := func(host string, port int, tag string) {
		host = strings.TrimSpace(host)
		if host == "" || port <= 0 {
			return
		}
		k := host + ":" + strconv.Itoa(port)
		if seen[k] {
			return
		}
		seen[k] = true
		out = append(out, addrCand{host: host, port: port, tag: tag})
	}
	add(g.Host, g.Port, "主")
	if g.ISP {
		add(g.CMCC, g.CMCCPort, "移动")
		add(g.CTCC, g.CTCCPort, "电信")
		add(g.CUCC, g.CUCCPort, "联通")
	}
	return out
}

func (g *GameServer) CandidateStrings() []string {
	cs := g.Candidates()
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.host+":"+strconv.Itoa(c.port))
	}
	return out
}

func (s *Session) candidates() []addrCand {
	var out []addrCand
	seen := map[string]bool{}
	push := func(host string, port int, tag string) {
		if host == "" || port <= 0 {
			return
		}
		k := host + ":" + strconv.Itoa(port)
		if seen[k] {
			return
		}
		seen[k] = true
		out = append(out, addrCand{host: host, port: port, tag: tag})
	}
	push(s.cfg.Host, s.cfg.Port, "主")
	for _, c := range s.cfg.Candidates {
		h, p := splitHostPort(c)
		if h == "" {
			continue
		}
		push(h, p, "候选")
	}
	return out
}

func splitHostPort(s string) (string, int) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", 0
	}
	i := strings.LastIndexByte(s, ':')
	if i < 0 {
		return s, 19132
	}
	host := s[:i]
	port, err := strconv.Atoi(s[i+1:])
	if err != nil || port <= 0 {
		return host, 19132
	}
	return host, port
}

func tryCloseRk(rk *raknet.Client) {
	if rk == nil {
		return
	}
	defer func() { _ = recover() }()
	rk.Close()
}
