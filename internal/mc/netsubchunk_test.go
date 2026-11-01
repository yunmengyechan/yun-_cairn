package mc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestNetworkSubChunkVsYechuan(t *testing.T) {
	repo := os.Getenv("PH_REPO")
	if repo == "" {
		t.Skip("没设 PH_REPO")
	}

	os.Chdir(repo)
	hexPath := filepath.Join(repo, "_netpayload.hex")
	jsonPath := filepath.Join(repo, "_netpayload.json")
	hb, err := os.ReadFile(hexPath)
	if err != nil {
		t.Skipf("没有载荷文件（先跑 _netc.py）: %v", err)
	}
	jb, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Skipf("没有期望结果: %v", err)
	}

	payload, err := hex.DecodeString(string(trimSpace(hb)))
	if err != nil {
		t.Fatalf("hex 解码: %v", err)
	}
	fmt.Printf("  载荷 %d 字节\n", len(payload))

	var exp [][]int64
	if err := json.Unmarshal(jb, &exp); err != nil {
		t.Fatalf("期望 JSON: %v", err)
	}
	fmt.Printf("  yechuan 期望 %d 个方块\n", len(exp))

	sp, err := ParseSubChunkPalette(payload)
	if err != nil {
		t.Fatalf("ParseSubChunkPalette: %v", err)
	}
	fmt.Printf("  我们调色板 %d 项 storages_end=%d version=%d yIndex=%d\n",
		len(sp.Hashes), sp.StoragesEnd, sp.Version, sp.YIndex)

	bs, _, err := DecodeSubChunk(payload)
	if err != nil {
		t.Fatalf("DecodeSubChunk: %v", err)
	}
	fmt.Printf("  我们解出 %d 个方块\n", len(bs))

	if len(bs) != len(exp) {
		t.Fatalf("数量不符：我们 %d，yechuan %d", len(bs), len(exp))
	}

	diff := 0
	for i := range exp {
		ex, ey, ez := int(exp[i][0]), int(exp[i][1]), int(exp[i][2])
		eh := uint32(exp[i][3])
		got := bs[i]
		if got.X != ex || got.Y != ey || got.Z != ez || got.Hash != eh {
			diff++
			if diff <= 5 {
				fmt.Printf("    差异 @%d: 我们 (%d,%d,%d,0x%08x) vs yechuan (%d,%d,%d,0x%08x)\n",
					i, got.X, got.Y, got.Z, got.Hash, ex, ey, ez, eh)
			}
		}
	}
	if diff > 0 {
		t.Errorf("%d/%d 个方块不一致", diff, len(exp))
	} else {
		fmt.Printf("  ✅ %d 个方块逐条一致（坐标 + 哈希）\n", len(bs))
	}

	if len(bs) > 0 {
		first := bs[0]
		last := bs[len(bs)-1]
		fmt.Printf("    首个 (%d,%d,%d) 0x%08x %s\n", first.X, first.Y, first.Z, first.Hash, first.Name)
		fmt.Printf("    末个 (%d,%d,%d) 0x%08x %s\n", last.X, last.Y, last.Z, last.Hash, last.Name)
	}

	if sp.StoragesEnd != len(payload) {
		t.Errorf("storages_end=%d 应等于载荷长度 %d", sp.StoragesEnd, len(payload))
	} else {
		fmt.Printf("  ✅ storages_end == 载荷长度 == %d（全部消耗）\n", len(payload))
	}
}

func trimSpace(b []byte) []byte {
	var out []byte
	for _, c := range b {
		if c != ' ' && c != '\n' && c != '\r' && c != '\t' {
			out = append(out, c)
		}
	}
	return out
}
