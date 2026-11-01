package mc

import (
	"encoding/hex"
	"fmt"
)

const HeartbeatT = 1748429465

type RpcContext struct {
	UID           string
	PlayerID      int64
	EngineVersion string
	PatchVersion  string
}

func BuildPyRpcFrame(payload []byte) []byte {
	b := []byte{IDPyRpc, 0x01}
	b = append(b, WriteVarint(uint32(len(payload)))...)
	b = append(b, payload...)
	b = append(b, 0, 0, 0, 0)
	return b
}

func rpcToBytes(v any) []byte {
	switch t := v.(type) {
	case string:
		return []byte(t)
	case []byte:
		return t
	default:
		return nil
	}
}

func HandlePyRpc(pkt []byte, ctx RpcContext) ([][]byte, string, error) {
	if len(pkt) < 3 {
		return nil, "", fmt.Errorf("PyRpc 太短")
	}
	off := 2
	rpcLen, n := ReadVarint(pkt, off)
	off += int(n)
	if off+int(rpcLen) > len(pkt) {
		return nil, "", fmt.Errorf("PyRpc 越界")
	}
	rpcData := pkt[off : off+int(rpcLen)]
	v, _, err := MPUnpack(rpcData)
	if err != nil {
		return nil, "", err
	}
	arr, ok := v.([]any)
	if !ok || len(arr) == 0 {
		return nil, "", fmt.Errorf("PyRpc 不是数组")
	}
	cmd, _ := arr[0].(string)
	var params []any
	if len(arr) > 1 {
		if p, ok := arr[1].([]any); ok {
			params = p
		}
	}

	switch cmd {
	case "S2CHeartBeat":
		reply := []any{"ClientLoadAddonsFinishedFromGac", []any{}, nil}
		return [][]byte{BuildPyRpcFrame(MPPack(reply))}, cmd, nil

	case "GetStartType":
		if len(params) == 0 {
			return nil, cmd, fmt.Errorf("GetStartType 缺参数")
		}
		raw, err := hex.DecodeString(string(rpcToBytes(params[0])))
		if err != nil {
			return nil, cmd, fmt.Errorf("GetStartType 参数不是 hex: %w", err)
		}
		data, err := UtilityDecryptWithTail(raw)
		if err != nil {
			return nil, cmd, fmt.Errorf("utility 解密失败: %w", err)
		}
		combined := append([]byte(ctx.UID), data...)
		enc, err := UtilityEncryptWithTail(combined)
		if err != nil {
			return nil, cmd, fmt.Errorf("utility 加密失败: %w", err)
		}
		reply := []any{"SetStartType", []any{hex.EncodeToString(enc)}, nil}
		return [][]byte{BuildPyRpcFrame(MPPack(reply))}, cmd, nil

	case "GetMCPCheckNum":
		out := [][]byte{}
		hb := []any{"C2SHeartBeat", []any{map[string]any{
			"is_64":      false,
			"t":          HeartbeatT,
			"is_android": false,
		}}, nil}
		out = append(out, BuildPyRpcFrame(MPPack(hb)))

		if len(params) > 0 {
			blob := rpcToBytes(params[0])
			salt := ""
			if len(params) > 1 {
				if sa, ok := params[1].([]any); ok && len(sa) > 0 {
					salt = string(rpcToBytes(sa[0]))
				}
			}
			reply, err := MCPCheckNum(blob, salt, int(ctx.PlayerID),
				ctx.EngineVersion, ctx.PatchVersion, ctx.UID)
			if err != nil {
				return out, cmd, fmt.Errorf("MCPCheckNum 失败: %w", err)
			}
			out = append(out, BuildPyRpcFrame(MPPack(reply)))
		}
		return out, cmd, nil
	}

	return nil, cmd, nil
}
