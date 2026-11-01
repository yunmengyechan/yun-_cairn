package mc

import (
	"encoding/binary"
	"math"
	"math/rand"
)

func BuildClientCacheStatus(enabled bool) []byte {
	e := byte(0)
	if enabled {
		e = 1
	}
	return []byte{IDClientCacheStatus, e, 0}
}

func BuildResourcePackResponse(response byte) []byte {
	return []byte{IDResourcePackClientResp, response, 0, 0}
}

const defaultChunkRadius = 6

func BuildRequestChunkRadius(radius, maxRadius byte) []byte {
	return append([]byte{IDRequestChunkRadius}, WriteVarint(uint32(radius))[0], maxRadius)

}

func BuildRequestAbility() []byte {
	b := []byte{IDRequestAbility}
	b = append(b, WriteVarint(2)...)
	b = append(b, WriteVarint(2)...)
	b = append(b, WriteVarint(0)...)
	return b
}

func BuildTickSync() []byte {
	b := []byte{IDTickSync}
	b = append(b, WriteVarint(0)...)
	b = append(b, WriteZigzag64(uint64(rand.Int63()))...)
	return b
}

func BuildPostStartGameHandshake() [][]byte {
	return [][]byte{
		BuildRequestChunkRadius(defaultChunkRadius, 12),
		BuildRequestAbility(),
		BuildTickSync(),
		BuildPyRpcE(),
		BuildPyRpcSyncUsingMod(),
	}
}

const PyRpcClientID = 0x05DB23AE

func BuildPyRpc(payload []byte) []byte {
	b := []byte{IDPyRpc}
	id := uint32(PyRpcClientID)
	b = append(b, byte(id>>24), byte(id>>16), byte(id>>8), byte(id))
	b = append(b, payload...)
	return b
}

func BuildPyRpcE() []byte {
	return BuildPyRpc([]byte{0x93, 0xA1, 0x65, 0x90, 0xC0})
}

func BuildPyRpcSyncUsingMod() []byte {
	p := []byte{0x93, 0xAC}
	p = append(p, []byte("SyncUsingMod")...)
	p = append(p, 0x95, 0x90, 0xDA, 0x00, 0x24)
	p = append(p, []byte("c18e65aa-7b21-4637-9b63-8ad63622ef01")...)
	p = append(p, 0xC0, 0xC2, 0x80, 0xC0)
	return BuildPyRpc(p)
}

func BuildPlayerAuthInput(pitch, yaw, x, y, z float32, tick int64) []byte {
	b := []byte{}
	b = append(b, WriteVarint(IDPlayerAuthInput)...)
	b = appendF32(b, pitch)
	b = appendF32(b, yaw)
	b = appendF32(b, x)
	b = appendF32(b, y)
	b = appendF32(b, z)
	b = appendF32(b, 0)
	b = appendF32(b, 0)
	b = appendF32(b, yaw)
	b = append(b, WriteVarint(0)...)
	b = append(b, WriteVarint(1)...)
	b = append(b, WriteVarint(0)...)
	b = append(b, WriteVarint(0)...)
	b = appendF32(b, 0)
	b = appendF32(b, 0)
	b = append(b, WriteZigzag64(uint64(tick))...)
	b = appendF32(b, 0)
	b = appendF32(b, 0)
	b = appendF32(b, 0)
	b = append(b, 0)
	b = appendF32(b, 0)
	b = appendF32(b, 0)
	b = appendF32(b, 0)
	b = appendF32(b, 0)
	b = appendF32(b, 0)
	b = appendF32(b, 0)
	b = appendF32(b, 0)
	b = append(b, 0)
	b = appendF32(b, 0)
	b = appendF32(b, 0)
	b = append(b, 0)
	b = append(b, 1)
	b = append(b, 0)
	return b
}

func appendF32(b []byte, v float32) []byte {
	var t [4]byte
	binary.LittleEndian.PutUint32(t[:], math.Float32bits(v))
	return append(b, t[:]...)
}
