package nbt

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRealOtherFormats(t *testing.T) {
	root := os.Getenv("PH_REPO")
	if root == "" {
		t.Skip("没设 PH_REPO")
	}
	d := filepath.Join(root, "build", "_format_samples")
	if _, err := os.Stat(d); err != nil {
		t.Skip("没有 _format_samples")
	}

	files, _ := filepath.Glob(filepath.Join(d, "*"))
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		base := filepath.Base(f)

		raw, _ := Decompress(b)

		var le, be error
		var cle, cbe Compound
		cle, le = Parse(raw, true)
		cbe, be = Parse(raw, false)

		pick := ""
		var c Compound
		if le == nil && be != nil {
			pick, c = "小端", cle
		} else if be == nil && le != nil {
			pick, c = "大端", cbe
		} else if le == nil && be == nil {

			if len(cle) >= len(cbe) {
				pick, c = "小端(两可)", cle
			} else {
				pick, c = "大端(两可)", cbe
			}
		} else {
			fmt.Printf("  %-26s %6d 字节  两种字节序都解不开\n", base, len(b))
			continue
		}

		keys := make([]string, 0, len(c))
		for k := range c {
			keys = append(keys, k)
		}
		sortStrings(keys)
		show := keys
		if len(show) > 6 {
			show = show[:6]
		}
		fmt.Printf("  %-26s %6d 字节  %-10s 字段=%-3d %v\n",
			base, len(b), pick, len(c), show)
	}
}

var _ = binary.LittleEndian
