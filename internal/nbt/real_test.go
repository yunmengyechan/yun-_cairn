package nbt

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRealMcStructure(t *testing.T) {
	root := os.Getenv("PH_REPO")
	if root == "" {
		t.Skip("没设 PH_REPO，跳过真实文件测试")
	}

	globs := []string{
		filepath.Join(root, "build", "_format_samples", "*.mcstructure"),
		filepath.Join(root, "mcpe_apk_extract", "assets", "assets", "behavior_packs",
			"vanilla", "structures", "bastion", "mobs", "*.mcstructure"),
	}

	var files []string
	for _, g := range globs {
		m, _ := filepath.Glob(g)
		files = append(files, m...)
	}
	if len(files) == 0 {
		t.Skip("找不到 mcstructure 样本")
	}

	ok := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("%s 读失败: %v", filepath.Base(f), err)
			continue
		}

		c, err := Parse(b, true)
		if err != nil {
			t.Errorf("%s 解析失败: %v", filepath.Base(f), err)
			continue
		}

		sz := c.GetList("size")
		st := c.GetCompound("structure")
		if st == nil {
			t.Errorf("%s 没有 structure 字段", filepath.Base(f))
			continue
		}

		pal := st.GetList("block_palette")
		if len(pal) == 0 {
			if pd := st.GetCompound("palette"); pd != nil {
				if def := pd.GetCompound("default"); def != nil {
					pal = def.GetList("block_palette")
				}
			}
		}
		idx := st.GetList("block_indices")
		origin := c.GetList("structure_world_origin")

		var dims []int64
		for _, v := range sz {
			dims = append(dims, toI64(v.Val))
		}
		var org []int64
		for _, v := range origin {
			org = append(org, toI64(v.Val))
		}

		layer0 := 0
		if len(idx) > 0 {
			if l0, okk := idx[0].Val.([]Tag); okk {
				layer0 = len(l0)
			}
		}

		name := ""
		if len(pal) > 0 {
			if pc, okk := pal[0].Val.(Compound); okk {
				name = pc.GetStr("name", "")
			}
		}

		fmt.Printf("  %-28s %6d 字节  size=%v  调色板=%d  主层=%d  origin=%v  首方块=%s\n",
			filepath.Base(f), len(b), dims, len(pal), layer0, org, name)
		ok++
	}
	fmt.Printf("  ✅ %d/%d 个 mcstructure 解析成功\n", ok, len(files))
}

func toI64(v any) int64 {
	switch x := v.(type) {
	case int8:
		return int64(x)
	case int16:
		return int64(x)
	case int32:
		return int64(x)
	case int64:
		return x
	}
	return 0
}
