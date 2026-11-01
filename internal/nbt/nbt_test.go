package nbt

import (
	"encoding/hex"
	"fmt"
	"os"
	"testing"
)

func TestDumpHex(t *testing.T) {
	le := os.Getenv("NBT_LE") != "0"
	sub := Compound{"x": Byte(1)}
	c := Compound{
		"byteVal":  Byte(-7),
		"byteArr":  ByteArray([]byte{1, 2, 250}),
		"dblVal":   Double(3.5),
		"fltVal":   Float(1.25),
		"intArr":   IntArray([]int32{10, -20, 30}),
		"intVal":   Int(-123456),
		"listVal":  ListOf(TagInt, []Tag{Int(1), Int(2), Int(3)}),
		"longArr":  LongArray([]int64{1 << 40, -(1 << 40)}),
		"longVal":  Long(1 << 45),
		"shortVal": Short(-300),
		"strVal":   String("hi 你好"),
		"sub":      Tag{Type: TagCompound, Val: sub},
	}

	b, err := Dump(c, le, "root")
	if err != nil {
		t.Fatalf("Dump: %v", err)
	}
	fmt.Println("HEX " + hex.EncodeToString(b))

	c2, err := Parse(b, le)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(c2) != len(c) {
		t.Fatalf("回读字段数不对：%d != %d", len(c2), len(c))
	}
	fmt.Printf("ROUNDTRIP-OK %d fields\n", len(c2))
}
