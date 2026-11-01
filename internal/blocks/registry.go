package blocks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Info struct {
	Name   string
	States string
}

var (
	once    sync.Once
	table   map[uint32]Info
	loadErr error
	srcPath string
)

func candidates() []string {
	var out []string
	if v := os.Getenv("PH_BLOCK_REGISTRY"); v != "" {
		out = append(out, v)
	}
	out = append(out,
		"block_registry.json",
		filepath.Join("yechuan", "core", "online", "block_registry.json"),
		filepath.Join("..", "yechuan", "core", "online", "block_registry.json"),
		filepath.Join("..", "..", "yechuan", "core", "online", "block_registry.json"),
	)
	return out
}

func Load() (map[uint32]Info, error) {
	once.Do(func() {
		var data []byte
		var err error
		for _, p := range candidates() {
			data, err = os.ReadFile(p)
			if err == nil {
				srcPath = p
				break
			}
		}
		if data == nil {
			loadErr = fmt.Errorf("找不到 block_registry.json，试过：%v", candidates())
			return
		}

		var raw map[string][]string
		if err := json.Unmarshal(data, &raw); err != nil {
			loadErr = fmt.Errorf("block_registry.json 解析失败: %w", err)
			return
		}

		table = make(map[uint32]Info, len(raw))
		for k, v := range raw {
			var h uint64
			if _, err := fmt.Sscanf(k, "%x", &h); err != nil {
				continue
			}
			it := Info{}
			if len(v) > 0 {
				it.Name = v[0]
			}
			if len(v) > 1 {
				it.States = v[1]
			}
			table[uint32(h)] = it
		}
	})
	return table, loadErr
}

func Lookup(h uint32) Info {
	t, err := Load()
	if err != nil || t == nil {
		return Info{Name: "minecraft:unknown", States: "{}"}
	}
	if v, ok := t[h]; ok {
		return v
	}
	return Info{Name: "minecraft:unknown", States: "{}"}
}

func Name(h uint32) string { return Lookup(h).Name }

func Size() int {
	t, _ := Load()
	return len(t)
}

func SourcePath() string { return srcPath }
