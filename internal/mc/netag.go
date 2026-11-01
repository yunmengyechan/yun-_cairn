package mc

import (
	"fmt"
	"os"
	"path/filepath"

	"cairn/internal/nbt"
)

type NETagPayload struct {
	ID   string
	Raw  []byte
	OK   bool
	Data map[string]any
}

func SplitNETag(c nbt.Compound) *NETagPayload {
	if c == nil {
		return nil
	}
	tagStr := c.GetStr("__tag", "")
	if tagStr == "" {
		return nil
	}
	id := c.GetStr("id", "")
	if id == "" {
		return nil
	}
	return &NETagPayload{
		ID:  id,
		Raw: []byte(tagStr),
	}
}

func UnpackNETag(p *NETagPayload) error {
	if p == nil {
		return fmt.Errorf("nil payload")
	}

	p.Data = map[string]any{
		"id":       p.ID,
		"tag_len":  len(p.Raw),
		"tag_hex":  hexHead(p.Raw, len(p.Raw)),
		"unparsed": true,
	}
	return nil
}

func DumpNETagSamples(p *NETagPayload) {
	if p == nil || len(p.Raw) == 0 {
		return
	}
	dir := filepath.Join(os.Getenv("APPDATA"), "PeHangfu")
	_ = os.MkdirAll(dir, 0o755)
	fn := filepath.Join(dir, "netag-samples.jsonl")
	f, err := os.OpenFile(fn, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	line := fmt.Sprintf("{\"id\":\"%s\",\"len\":%d,\"hex\":\"%s\"}\n",
		p.ID, len(p.Raw), hexHead(p.Raw, len(p.Raw)))
	_, _ = f.WriteString(line)
}

func hexHead(b []byte, n int) string {
	if len(b) < n {
		n = len(b)
	}
	const hexd = "0123456789abcdef"
	out := make([]byte, 0, n*2)
	for _, v := range b[:n] {
		out = append(out, hexd[v>>4], hexd[v&0x0f])
	}
	return string(out)
}
