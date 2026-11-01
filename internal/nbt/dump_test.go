package nbt

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestDumpRealFiles(t *testing.T) {
	repo := os.Getenv("PH_REPO")
	if repo == "" {
		t.Skip("没设 PH_REPO")
	}
	os.Chdir(repo)

	type item struct {
		label  string
		path   string
		little bool
	}
	var items []item
	if f, _ := filepath.Glob(filepath.Join("build", "_format_samples", "*.mcstructure")); len(f) > 0 {
		items = append(items, item{"mcstructure（小端裸 NBT）", f[0], true})
	}
	if f, _ := filepath.Glob(filepath.Join("build", "_format_samples", "*.litematic")); len(f) > 0 {
		items = append(items, item{"litematic（大端 gzip）", f[0], false})
	}
	if f, _ := filepath.Glob(filepath.Join("build", "_format_samples", "*.schematic")); len(f) > 0 {
		items = append(items, item{"schematic（大端 gzip）", f[0], false})
	}
	if f, _ := filepath.Glob(filepath.Join("mcpe_apk_extract", "assets", "assets", "behavior_packs",
		"vanilla", "structures", "bastion", "mobs", "*.mcstructure")); len(f) > 0 {
		items = append(items, item{"官方原版 mcstructure", f[0], true})
	}

	for _, it := range items {
		raw, err := os.ReadFile(it.path)
		if err != nil {
			continue
		}
		fmt.Printf("\n=== %s ===\n", it.label)
		fmt.Printf("  文件: %s (%d 字节)\n", filepath.Base(it.path), len(raw))

		dec, err := Decompress(raw)
		if err != nil {
			t.Errorf("  解压失败: %v", err)
			continue
		}
		if len(dec) != len(raw) {
			fmt.Printf("  解压后: %d 字节\n", len(dec))
		}

		rootName, c, err := ParseNamed(dec, it.little)
		if err != nil {
			t.Errorf("  解析失败: %v", err)
			continue
		}
		fmt.Printf("  根名: %q   顶层标签数: %d\n", rootName, len(c))
		dumpCompound(c, "  ", 3)
	}
}

func dumpCompound(c Compound, indent string, maxDepth int) {
	if maxDepth <= 0 {
		return
	}
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		dumpTag(k, c[k], indent, maxDepth)
	}
}

func dumpTag(name string, t Tag, indent string, depth int) {
	switch v := t.Val.(type) {
	case Compound:
		fmt.Printf("%s%s: <compound %d 项>\n", indent, name, len(v))
		dumpCompound(v, indent+"  ", depth-1)
	case []Tag:
		fmt.Printf("%s%s: <list[%s] %d 项>\n", indent, name, TagNames[t.Elem], len(v))
		for i, e := range v {
			if i >= 3 {
				fmt.Printf("%s  ...（共 %d 项）\n", indent, len(v))
				break
			}
			dumpTag(fmt.Sprintf("[%d]", i), e, indent+"  ", depth-1)
		}
	case []byte:
		fmt.Printf("%s%s: <byte[] %d 字节>\n", indent, name, len(v))
	case []int32:
		fmt.Printf("%s%s: <int[] %d 项>\n", indent, name, len(v))
	case []int64:
		fmt.Printf("%s%s: <long[] %d 项>\n", indent, name, len(v))
	case string:
		show := v
		if len(show) > 50 {
			show = show[:50] + "..."
		}
		fmt.Printf("%s%s: %q\n", indent, name, show)
	default:
		fmt.Printf("%s%s: %v  (%s)\n", indent, name, v, TagNames[t.Type])
	}
}

var _ = strings.TrimSpace
