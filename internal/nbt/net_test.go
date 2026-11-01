package nbt

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestNetworkVsPython(t *testing.T) {
	f := os.Getenv("NET_NBT_FILE")
	if f == "" {
		t.Skip("没设 NET_NBT_FILE")
	}
	raw, err := os.ReadFile(f)
	if err != nil {
		t.Fatalf("读文件: %v", err)
	}
	b, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("hex 解码: %v", err)
	}
	_, c, n, err := ParseNetwork(b)
	if err != nil {
		t.Fatalf("ParseNetwork: %v", err)
	}
	m := map[string]any{}
	for k, tag := range c {
		m[k] = tag.Val
	}
	j, _ := json.MarshalIndent(m, "", "  ")
	fmt.Printf("GO consumed=%d fields=%d\n%s\n", n, len(c), string(j))
}
