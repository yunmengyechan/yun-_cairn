package mc

import (
	"crypto/ecdsa"
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	IDLogin                   = 0x01
	IDPlayStatus              = 0x02
	IDServerToClientHandshake = 0x03
	IDClientToServerHandshake = 0x04
	IDDisconnect              = 0x05
	IDResourcePacksInfo       = 0x06
	IDResourcePackClientResp  = 0x08
	IDText                    = 0x09
	IDStartGame               = 0x0b
	IDCommandRequest          = 0x4d
	IDCommandOutput           = 0x4f
	IDClientCacheStatus       = 0x81
	IDSettingsCommand         = 0x8c
	IDNetworkSettings         = 0x8f
	IDRequestNetworkSettings  = 0xc1
)

const ProtocolVersion = 898

var protocolVersionOverride uint32

func SetProtocolVersion(v uint32) { protocolVersionOverride = v }

func curProtocolVersion() uint32 {
	if protocolVersionOverride != 0 {
		return protocolVersionOverride
	}
	return ProtocolVersion
}

const (
	IDMobEffect          = 0x1c
	IDTickSync           = 0x17
	IDRequestChunkRadius = 0x45
	IDRequestAbility     = 0xB8
	IDPyRpc              = 0xC8
	IDPlayerAuthInput    = 0x90

	IDNetworkChunkPublisherUpdate = 121
	IDSubChunk                    = 174
	IDSubChunkRequest             = 175
	IDItemStackRequest            = 147
	IDInventoryTransaction        = 30
	IDContainerClose              = 47
	IDCommandBlockUpdate          = 78
	IDMovePlayer                  = 0x13

	IDCorrectPlayerMovePrediction = 161

	IDSetHealth        = 0x2a
	IDRespawn          = 0x2d
	IDContainerOpen    = 0x2e
	IDInventoryContent = 0x31
	IDSubChunk2        = 0xae
	IDBlockActorData   = 0x38
	IDDeathInfo        = 0x3d

	IDPlayerList = 0x3f
)

func BuildRequestNetworkSettings() []byte {
	b := WriteVarint(IDRequestNetworkSettings)
	var p [4]byte
	binary.BigEndian.PutUint32(p[:], curProtocolVersion())
	b = append(b, p[:]...)
	return b
}

func RawPublicPoint(k *ecdsa.PrivateKey) []byte {
	out := make([]byte, 97)
	out[0] = 4
	k.X.FillBytes(out[1:49])
	k.Y.FillBytes(out[49:97])
	return out
}

func BuildClientToServerHandshake(pubRaw []byte) []byte {
	if len(pubRaw) == 0 {
		return []byte{IDClientToServerHandshake}
	}
	out := make([]byte, 0, 1+5+len(pubRaw))
	out = append(out, IDClientToServerHandshake)
	out = append(out, WriteVarint(uint32(len(pubRaw)))...)
	out = append(out, pubRaw...)
	return out
}

func buildClientToServerHandshakeEmpty() []byte {
	return []byte{IDClientToServerHandshake}
}

func BuildLogin(chain string, skinJWT string) []byte {
	chainB := []byte(chain)
	skinB := []byte(skinJWT)
	bodySize := 4 + len(chainB) + 4 + len(skinB)

	b := []byte{IDLogin}
	var p [4]byte
	binary.BigEndian.PutUint32(p[:], curProtocolVersion())
	b = append(b, p[:]...)
	b = append(b, WriteVarint(uint32(bodySize))...)

	var l4 [4]byte
	binary.LittleEndian.PutUint32(l4[:], uint32(len(chainB)))
	b = append(b, l4[:]...)
	b = append(b, chainB...)
	binary.LittleEndian.PutUint32(l4[:], uint32(len(skinB)))
	b = append(b, l4[:]...)
	b = append(b, skinB...)
	return b
}

type NetworkSettings struct {
	CompressionThreshold int
	CompressionAlgorithm int
}

func ParseNetworkSettings(pkt []byte) (*NetworkSettings, error) {
	id, n := ReadVarint(pkt, 0)
	off := int(n)
	if id != IDNetworkSettings {
		return nil, fmt.Errorf("不是 NetworkSettings: 0x%02x", id)
	}
	if off+4 > len(pkt) {
		return nil, errors.New("NetworkSettings 数据不足")
	}
	return &NetworkSettings{
		CompressionThreshold: int(binary.LittleEndian.Uint16(pkt[off : off+2])),
		CompressionAlgorithm: int(binary.LittleEndian.Uint16(pkt[off+2 : off+4])),
	}, nil
}

func ParsePlayStatus(pkt []byte) (uint32, error) {
	id, n := ReadVarint(pkt, 0)
	off := int(n)
	if id != IDPlayStatus {
		return 0, fmt.Errorf("不是 PlayStatus: 0x%02x", id)
	}
	if off+4 > len(pkt) {
		return 0, errors.New("PlayStatus 数据不足")
	}
	return binary.BigEndian.Uint32(pkt[off : off+4]), nil
}
