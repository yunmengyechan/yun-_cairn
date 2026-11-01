package blocks

import (
	"fmt"
	"os"
	"testing"
)

func TestRegistry(t *testing.T) {

	if os.Getenv("PH_REPO") != "" {
		os.Chdir(os.Getenv("PH_REPO"))
	}
	sz := Size()
	if sz == 0 {
		t.Skipf("注册表没加载到（路径 %q），跳过", SourcePath())
	}
	fmt.Printf("  注册表条目数: %d  来源: %s\n", sz, SourcePath())

	cases := []struct {
		h    uint32
		want string
	}{
		{0x09129f50, "minecraft:acacia_button"},
		{0x05689749, "minecraft:acacia_button"},
	}
	for _, c := range cases {
		got := Name(c.h)
		if got != c.want {
			t.Errorf("0x%08x: 得到 %q，期望 %q", c.h, got, c.want)
		} else {
			fmt.Printf("  0x%08x -> %s\n", c.h, got)
		}
	}

	if n := Name(0xDEADBEEF); n != "minecraft:unknown" {
		t.Errorf("未命中的哈希应返回 minecraft:unknown，得到 %q", n)
	} else {
		fmt.Println("  0xdeadbeef -> minecraft:unknown（符合预期）")
	}
}
