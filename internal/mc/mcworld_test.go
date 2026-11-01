package mc

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"cairn/internal/leveldb"
)

func TestMcWorldEndToEnd(t *testing.T) {
	repo := os.Getenv("PH_REPO")
	if repo == "" {
		t.Skip("没设 PH_REPO")
	}
	os.Chdir(repo)

	mws, _ := filepath.Glob(filepath.Join(repo, "build", "_format_samples", "*.mcworld"))
	if len(mws) == 0 {
		t.Skip("没找到 mcworld 样本")
	}

	totalBlocks := 0
	for _, mw := range mws {
		zr, err := zip.OpenReader(mw)
		if err != nil {
			continue
		}
		fmt.Printf("  === %s ===\n", filepath.Base(mw))

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

			tb, err := leveldb.Open(data)
			if err != nil {
				t.Errorf("  %s: %v", f.Name, err)
				continue
			}

			chunks := 0
			blocks := 0
			err = tb.Iterate(func(k, v []byte) bool {
				ck, ok := leveldb.ParseChunkKey(k)
				if !ok {
					return true
				}

				if ck.Tag != leveldb.TagSubChunk && ck.Tag != leveldb.TagSubChunkV2 {
					return true
				}
				chunks++
				subY := 0
				if ck.SubY != nil {
					subY = int(*ck.SubY)
				}
				bs, derr := DecodeDiskSubChunk(v, subY, 4096)
				if derr != nil {
					fmt.Printf("      区块(%d,%d,%d) 解码失败: %v\n", ck.X, ck.Z, subY, derr)
					return true
				}
				if chunks <= 3 {
					names := map[string]int{}
					for _, b := range bs {
						names[b.Name]++
					}
					fmt.Printf("      区块(%d,%d) subY=%d  tag=%s  %d 字节 → %d 个方块\n",
						ck.X, ck.Z, subY, leveldb.TagName(ck.Tag), len(v), len(bs))
					shown := 0
					for n, c := range names {
						if shown >= 4 {
							break
						}
						fmt.Printf("          %s × %d\n", n, c)
						shown++
					}
				}
				blocks += len(bs)
				return true
			})
			if err != nil {
				fmt.Printf("      （遍历报错: %v）\n", err)
			}
			fmt.Printf("      %s: %d 个子区块，共 %d 个方块\n", f.Name, chunks, blocks)
			totalBlocks += blocks
		}
		zr.Close()
	}
	fmt.Printf("\n  合计解出 %d 个方块\n", totalBlocks)
}
