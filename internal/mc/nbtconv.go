package mc

import "cairn/internal/nbt"

func nbtToAny(c nbt.Compound) map[string]any {
	out := make(map[string]any, len(c))
	for k, t := range c {
		out[k] = nbtVal(t)
	}
	return out
}

func nbtVal(t nbt.Tag) any {
	switch v := t.Val.(type) {
	case nbt.Compound:
		return nbtToAny(v)
	case []nbt.Tag:
		out := make([]any, 0, len(v))
		for _, e := range v {
			out = append(out, nbtVal(e))
		}
		return out
	default:
		return v
	}
}
