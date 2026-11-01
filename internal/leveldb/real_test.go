package leveldb

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestRealMcWorld(t *testing.T) {
	repo := os.Getenv("PH_REPO")
	if repo == "" {
		t.Skip("没设 PH_REPO")
	}
	cands, _ := filepath.Glob(filepath.Join(repo, "build", "_format_samples", "*.mcworld"))
	if len(cands) == 0 {
		t.Skip("没找到 mcworld 样本")
	}

	for _, mw := range cands {
		zr, err := zip.OpenReader(mw)
		if err != nil {
			t.Logf("  %s 不是 ZIP: %v", filepath.Base(mw), err)
			continue
		}
		fmt.Printf("  === %s ===\n", filepath.Base(mw))
		for _, f := range zr.File {
			fmt.Printf("    %s (%d 字节)\n", f.Name, f.UncompressedSize64)
		}

		for _, f := range zr.File {
			if filepath.Ext(f.Name) != ".ldb" {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				continue
			}
			data, _ := io.ReadAll(rc)
			rc.Close()

			tb, err := Open(data)
			if err != nil {
				t.Errorf("  %s Open 失败: %v", f.Name, err)
				continue
			}
			fmt.Printf("    %s: footer=%d 字节  index@%d+%d\n",
				f.Name, tb.FooterSize, tb.indexHandle.Offset, tb.indexHandle.Size)

			type kv struct {
				k   string
				tag byte
			}
			var got []kv
			nerr := tb.Iterate(func(k, v []byte) bool {
				if ck, ok := ParseChunkKey(k); ok {
					got = append(got, kv{
						k:   fmt.Sprintf("(%d,%d) %s subY=%s", ck.X, ck.Z, TagName(ck.Tag), subYStr(ck.SubY)),
						tag: ck.Tag,
					})
				} else {
					got = append(got, kv{k: fmt.Sprintf("%q", string(k))})
				}
				return true
			})
			if nerr != nil {
				fmt.Printf("      （遍历中报错，已读 %d 条: %v）\n", len(got), nerr)
			}
			fmt.Printf("      读出 %d 条键值\n", len(got))
			sort.Slice(got, func(i, j int) bool { return got[i].k < got[j].k })
			for i, e := range got {
				if i >= 8 {
					fmt.Printf("      ...（共 %d 条）\n", len(got))
					break
				}
				fmt.Printf("      %s\n", e.k)
			}
		}
		zr.Close()
		fmt.Println()
	}
}

func subYStr(p *int8) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *p)
}
