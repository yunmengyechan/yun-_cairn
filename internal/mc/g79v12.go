package mc

import (
	"encoding/hex"
	"fmt"
)

var G79V12Keys = [16]string{
	"60F1E0D1FD635362430747215CF1C2FF",
	"EA5B62D27D0338374852C4B9469D7AC6",
	"17238D55501C5F020B155FB3303591E6",
	"8C5CEAE0F443E006A050266F73ADD5B0",
	"1C02CE22FB22F0E72060217418F351F3",
	"9A01773FEBB0CFE0EBDBF37F4D23C27F",
	"43F32300BF252CC320E2572ACE766367",
	"07F161011B3101F1ED0301735631E734",
	"0454E7707A5F37565601E100406060AF",
	"647554BAD3100C43C16660F002CC10F3",
	"E157213170F842382032564265B0B043",
	"914FC59311B04151393EF6896A847636",
	"0710C0205D224237025323265C145FA1",
	"054E6F01165267025C3111F562A921E9",
	"722D1789E792E2CA0D5322211FD0F5AE",
	"91F7C751FCF671F34943430772341799",
}

func PickG79V12Key(tail byte) []byte {
	idx := (tail >> 4) & 0x0F
	k, _ := hex.DecodeString(G79V12Keys[idx])
	return k
}

func VQStripZeroPadding(d []byte) []byte {
	num := 0
	num2 := len(d) - 1
	for num < 16 && num2 >= 0 {
		if d[num2] != 0 {
			num++
		}
		num2--
	}
	if num2+1 < 0 {
		return []byte{}
	}
	return d[:num2+1]
}

func UtilityEncryptWithTail(body []byte) ([]byte, error) {
	if len(body) == 0 {
		return []byte{}, nil
	}
	need := len(body) + 16
	total := ((need + 15) / 16) * 16
	plain := make([]byte, total)
	copy(plain, body)
	copy(plain[len(body):], randASCII(16))

	keyIdx := randIntn(16)
	tail := byte((keyIdx << 4) | 0x04)
	iv := randASCII(16)
	key := PickG79V12Key(tail)

	ct, err := aesCBCEncrypt(key, iv, plain)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, 16+len(ct)+1)
	out = append(out, iv...)
	out = append(out, ct...)
	out = append(out, tail)
	return out, nil
}

func UtilityDecryptWithTail(body []byte) ([]byte, error) {
	if len(body) < 33 {
		return nil, fmt.Errorf("utility_decrypt_with_tail: 数据太短 %d", len(body))
	}
	if len(body)%16 != 1 {
		return nil, fmt.Errorf("utility_decrypt_with_tail: 长度不合法 %d", len(body))
	}
	aeslen := len(body) - 17
	if aeslen <= 0 || aeslen%16 != 0 {
		return nil, fmt.Errorf("utility_decrypt_with_tail: 密文长度不合法 %d", aeslen)
	}
	iv := body[:16]
	tail := body[len(body)-1]
	ct := body[16 : len(body)-1]
	key := PickG79V12Key(tail)
	dec, err := aesCBCDecrypt(key, iv, ct)
	if err != nil {
		return nil, err
	}
	return VQStripZeroPadding(dec), nil
}
