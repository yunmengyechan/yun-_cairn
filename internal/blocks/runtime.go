package blocks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

var (
	rtOnce  sync.Once
	rtTable map[int]Info
	rtErr   error
	rtPath  string
)

func runtimeCandidates() []string {
	var out []string
	if v := os.Getenv("PH_RUNTIME_IDS"); v != "" {
		out = append(out, v)
	}
	bases := []string{
		filepath.Join("Exchange Tool.exe_extracted", "package", "MCStructureManage", "StructureBDX"),
		filepath.Join("..", "Exchange Tool.exe_extracted", "package", "MCStructureManage", "StructureBDX"),
		filepath.Join("..", "..", "Exchange Tool.exe_extracted", "package", "MCStructureManage", "StructureBDX"),
	}
	names := []string{"block_runtime_ids_1_19_10_22.json", "runtimeIds_117.json"}
	for _, b := range bases {
		for _, n := range names {
			out = append(out, filepath.Join(b, n))
		}
	}
	return out
}

func RuntimeTable() (map[int]Info, error) {
	rtOnce.Do(func() {
		var data []byte
		var err error
		for _, p := range runtimeCandidates() {
			data, err = os.ReadFile(p)
			if err == nil {
				rtPath = p
				break
			}
		}
		if data == nil {
			rtErr = fmt.Errorf("找不到运行时 ID 表，试过：%v", runtimeCandidates())
			return
		}

		var raw []json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			rtErr = fmt.Errorf("运行时 ID 表解析失败: %w", err)
			return
		}

		rtTable = make(map[int]Info, len(raw))
		for i, el := range raw {
			var pair []json.RawMessage
			if err := json.Unmarshal(el, &pair); err != nil || len(pair) == 0 {
				continue
			}
			var name string
			if err := json.Unmarshal(pair[0], &name); err != nil {
				continue
			}
			states := "{}"
			if len(pair) > 1 {

				var asStr string
				if json.Unmarshal(pair[1], &asStr) == nil {
					states = asStr
				} else {
					var asNum json.Number
					if json.Unmarshal(pair[1], &asNum) == nil {
						states = asNum.String()
					}
				}
			}
			rtTable[i] = Info{Name: normalizeRuntimeName(name), States: states}
		}
	})
	return rtTable, rtErr
}

func normalizeRuntimeName(n string) string {
	if len(n) > 5 && n[:5] == "tile." {
		n = n[5:]
	}

	for i := 0; i < len(n); i++ {
		if n[i] == ':' {
			return n
		}
	}
	return "minecraft:" + n
}

func RuntimeLookup(rid int) Info {
	t, err := RuntimeTable()
	if err != nil || t == nil {
		return Info{Name: "minecraft:unknown", States: "{}"}
	}
	if v, ok := t[rid]; ok {
		return v
	}

	return Info{Name: "unknown(rid=" + strconv.Itoa(rid) + ")", States: "{}"}
}

func RuntimeSize() int {
	t, _ := RuntimeTable()
	return len(t)
}

func RuntimeSourcePath() string { return rtPath }
