package mc

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const PeCoreServer = "https://g79obtcore.minecraft.cn:8443"

var signShiftTable = [5]uint32{218760705, 235472130, 51054340, 84609027, 235603713}

var signConstantTable = [40]uint32{
	4129170786, 3225465664, 643717713, 3921069994, 2763975236,
	1272893353, 4139469664, 3200236656, 1700485571, 2399980690,
	4293915773, 2240044497, 681279174, 3936430074, 3572445317,
	76029189, 568446438, 3275163606, 4107603335, 1163531501,
	4129170786, 3225465664, 643717713, 3921069994, 2763975236,
	1272893353, 4139469664, 3200236656, 1700485571, 2399980690,
	4293915773, 2240044497, 681279174, 3936430074, 3572445317,
	76029189, 568446438, 3275163606, 4107603335, 1163531501,
}

func PeSignCompute(message string, offset, rounds int) []byte {
	if message == "" {
		return nil
	}
	for len(message)%4 != 0 {
		message += "0"
	}
	b := []byte(message)
	list := make([]uint32, 0, len(b)/4+64)
	for i := 0; i < len(b); i += 4 {
		list = append(list, uint32(b[i])<<24|uint32(b[i+1])<<16|uint32(b[i+2])<<8|uint32(b[i+3]))
	}
	for len(list)%64 != 0 {
		list = append(list, 2882398599)
	}

	num := signShiftTable[offset]
	s1 := num & 0xFF
	s2 := (num >> 8) & 0xFF
	s3 := (num >> 16) & 0xFF
	s4 := (num >> 24) & 0xFF
	o := offset * 4
	c1 := signConstantTable[o]
	c2 := signConstantTable[o+1]
	c3 := signConstantTable[o+2]
	c4 := signConstantTable[o+3]

	a := uint32(1732584193)
	bb := uint32(4023233417)
	cc := uint32(2562383102)
	dd := uint32(271733878)

	for j := 0; j < rounds; j++ {
		for k := 0; k < len(list); k += 4 {
			v := list[k]
			t1 := v + c1 + ((bb & cc) | (dd & ^bb)) + a
			a = bb + (t1 << s1)
			t2 := a + v + c2 + ((cc & ^dd) | (dd & bb))
			bb += t2 << s2
			t3 := a + v + c3 + (bb ^ dd ^ cc)
			cc = bb + (t3 << s3)
			t4 := a + v + c4 + (cc ^ (bb | dd))
			dd = bb + (t4 << s4)
		}
	}

	out := make([]byte, 16)
	binary.LittleEndian.PutUint32(out[0:], a)
	binary.LittleEndian.PutUint32(out[4:], bb)
	binary.LittleEndian.PutUint32(out[8:], cc)
	binary.LittleEndian.PutUint32(out[12:], dd)
	return out
}

func HttpEncryptG79V12(bodyIn []byte) ([]byte, error) {
	total := ((len(bodyIn) + 16 + 15) / 16) * 16
	plain := make([]byte, total)
	copy(plain, bodyIn)
	copy(plain[len(bodyIn):], randASCII(16))
	tail := byte((randIntn(15) << 4) | 0x0C)
	iv := randASCII(16)
	ct, err := aesCBCEncrypt(PickG79V12Key(tail), iv, plain)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, 16+len(ct)+1)
	out = append(out, iv...)
	out = append(out, ct...)
	out = append(out, tail)
	return out, nil
}

func HttpDecryptG79V12(body []byte) (string, error) {
	if len(body) < 18 {
		return "", fmt.Errorf("响应太短 %d", len(body))
	}
	key := PickG79V12Key(body[len(body)-1])
	iv := body[:16]
	ct := body[16 : len(body)-1]
	arr, err := aesCBCDecrypt(key, iv, ct)
	if err != nil {
		return "", err
	}
	n := 0
	p := len(arr) - 1
	for n < 16 && p >= 0 {
		if arr[p] != 0 {
			n++
		}
		p--
	}
	if p+1 < 0 {
		return "", fmt.Errorf("解出来全是空")
	}
	return string(arr[:p+1]), nil
}

func HttpDecryptG79V12Large(body []byte) (string, error) {
	if len(body) < 18 {
		return "", fmt.Errorf("响应太短 %d", len(body))
	}
	key := PickG79V12Key(body[len(body)-1])
	iv := body[:16]
	ct := body[16 : len(body)-1]
	arr, err := aesCBCDecrypt(key, iv, ct)
	if err != nil {
		return "", err
	}
	num := len(arr) - 1
	for num >= 0 && arr[num] == 0 {
		num--
	}
	if num-127 < 0 {
		return "", fmt.Errorf("Large 变体长度不足 (num=%d)", num)
	}
	return string(arr[:num-127]), nil
}

func peUdidPath() string {
	exe, err := os.Executable()
	if err != nil {
		exe = "."
	}
	return filepath.Join(filepath.Dir(exe), "pe_udid.txt")
}

var peUdidCache string

func GetPeUdid() string {
	if peUdidCache != "" {
		return peUdidCache
	}
	p := peUdidPath()
	if b, err := os.ReadFile(p); err == nil {
		s := strings.TrimSpace(string(b))
		if len(s) == 32 {
			peUdidCache = s
			return s
		}
	}
	buf := make([]byte, 16)
	for i := range buf {
		buf[i] = byte(randIntn(256))
	}
	s := strings.ToUpper(hex.EncodeToString(buf))
	_ = os.WriteFile(p, []byte(s), 0o600)
	peUdidCache = s
	return s
}

func BuildPeSaData(udid string) string {

	const tpl = `{"app_channel":"netease","app_ver":"3.10.6.301389","core_num":"8","cpu_digit":"64","cpu_hz":"3200000","cpu_name":"placeholder","device_height":"1920","device_model":"REDMI 23113RKC6C","device_width":"1080","disk":"","emulator":0,"first_udid":"__U__","is_guest":0,"launcher_type":"PE_C++","mac_addr":"02:00:00:00:00:00","network":"CHANNEL_UNKNOW","os_name":"android","os_ver":"12","ram":"6238896128","rom":"134208294912","root":false,"sdk_ver":"5.16.0","start_type":"default","tdid":"","udid":"__U__"}`
	return strings.ReplaceAll(tpl, "__U__", udid)
}

func NormalizeCookieToSauth(cookie string) (string, error) {
	t := strings.TrimSpace(cookie)
	if t == "" {
		return "", fmt.Errorf("cookie 不能为空")
	}
	if strings.HasPrefix(t, "SDK4399") {
		b, err := base64.StdEncoding.DecodeString(t[7:])
		if err != nil {
			b, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(t[7:], "="))
		}
		if err != nil {
			return "", fmt.Errorf("SDK4399 后面的内容不是合法 base64")
		}
		return string(b), nil
	}
	if strings.HasPrefix(t, "ey") {
		b, err := base64.StdEncoding.DecodeString(t)
		if err != nil {
			b, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(t, "="))
		}
		if err == nil {
			return string(b), nil
		}
		return t, nil
	}
	var o map[string]any
	if err := json.Unmarshal([]byte(t), &o); err == nil {
		return t, nil
	}
	return "", fmt.Errorf("cookie 格式无效（既不是 base64 也不是 JSON）")
}

var peCoreHTTP = &http.Client{Timeout: 25 * time.Second}

type PeLoginResult struct {
	UserID   string
	Token    string
	Nickname string
	Avatar   string

	Sead string
}

func PeLoginWithSauth(sauth string) (*PeLoginResult, error) {

	engine := "3.10.5.301327"
	patch := "3.10.6.301389"
	k1 := "393dfdf6f78831f80a7e6aa305b21227"
	k2 := "c91713f56bbba226e4e8120c2d7a9ac42b3e7ca013bb30a74d822579860c042b"

	seed := randomGUID()
	msg := engine + k1 + patch + k2 + seed

	sign := base64.StdEncoding.EncodeToString(PeSignCompute(msg, 2, 8))
	if sign == "" {
		return nil, fmt.Errorf("PE 签名计算失败")
	}

	inner := sauth
	var o map[string]any
	if json.Unmarshal([]byte(sauth), &o) == nil {
		if s, ok := o["sauth_json"].(string); ok && s != "" {
			inner = s
		}
	}
	var io2 map[string]any
	if json.Unmarshal([]byte(inner), &io2) == nil {

		io2["platform"] = "ad"
		io2["source_platform"] = "ad"
		io2["sdk_version"] = "5.16.0"
		if _, ok := io2["is_unisdk_guest"]; !ok {
			io2["is_unisdk_guest"] = 0
		}
		if _, ok := io2["get_access_token"]; !ok {
			io2["get_access_token"] = "1"
		}
		if _, ok := io2["source_app_channel"]; !ok {
			if ch, ok2 := io2["app_channel"].(string); ok2 {
				io2["source_app_channel"] = ch
			}
		}
		if _, ok := io2["step"]; !ok {
			io2["step"] = "0"
		}
		if _, ok := io2["step2"]; !ok {
			io2["step2"] = "0"
		}
		if b, err := json.Marshal(io2); err == nil {
			inner = string(b)
		}
	}

	var saData, sauthRaw any
	if err := json.Unmarshal([]byte(BuildPeSaData(GetPeUdid())), &saData); err != nil {
		return nil, fmt.Errorf("sa_data 构造失败: %v", err)
	}
	if err := json.Unmarshal([]byte(inner), &sauthRaw); err != nil {
		return nil, fmt.Errorf("sauth_json 不是合法 JSON: %v", err)
	}

	body := map[string]any{
		"sa_data":        saData,
		"engine_version": engine,
		"patch_version":  patch,
		"message":        msg,
		"sauth_json":     sauthRaw,
		"seed":           seed,
		"sign":           sign,

		"extra_param": "extra",
		"pay_channel": "dashen_cloudgame",
	}
	jb, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	if os.Getenv("PH_DEBUG") != "" {
		fmt.Printf("[DBG] 请求体 %d 字节:\n%s\n", len(jb), string(jb))
	}
	enc, err := HttpEncryptG79V12(jb)
	if err != nil {
		return nil, err
	}
	hexBody := hex.EncodeToString(enc)

	req, err := http.NewRequest("POST", PeCoreServer+"/pe-authentication", strings.NewReader(hexBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := peCoreHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if len(raw) == 0 {
		return nil, fmt.Errorf("认证请求无响应 (HTTP %d)", resp.StatusCode)
	}

	ct := raw
	if hx, err := hex.DecodeString(strings.TrimSpace(string(raw))); err == nil {
		ct = hx
	}
	if os.Getenv("PH_DEBUG") != "" {
		fmt.Printf("[DBG] 原始响应 %d 字节，hex=%s\n", len(raw), string(raw))
	}
	plain, err := HttpDecryptG79V12(ct)
	if os.Getenv("PH_DEBUG") != "" {
		if err != nil {
			fmt.Printf("[DBG] 解密失败: %v\n", err)
		} else {
			fmt.Printf("[DBG] 解密后: %s\n", plain)
		}
	}
	if err != nil || !strings.HasPrefix(strings.TrimSpace(plain), "{") {
		p2, e2 := HttpDecryptG79V12Large(ct)
		if e2 == nil && strings.HasPrefix(strings.TrimSpace(p2), "{") {
			plain = p2
		} else if err != nil {
			return nil, fmt.Errorf("响应解密失败: %v", err)
		}
	}

	var out struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Entity  struct {
			EntityID string `json:"entity_id"`
			Token    string `json:"token"`
			Sead     string `json:"sead"`
		} `json:"entity"`
	}

	if os.Getenv("PH_DEBUG") != "" {
		fmt.Printf("[DBG] cairn plain len=%d\n[DBG] %s\n", len(plain), truncate(plain, 600))
	}
	dec := json.NewDecoder(strings.NewReader(plain))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("响应不是 JSON: %v | %s", err, truncate(plain, 200))
	}
	if out.Code != 0 {
		m := out.Message
		if m == "" {
			m = fmt.Sprintf("PE 登录失败 code=%d", out.Code)
		}
		return nil, fmt.Errorf("%s", m)
	}
	if out.Entity.EntityID == "" || out.Entity.Token == "" {
		return nil, fmt.Errorf("响应缺少 entity_id 或 token")
	}

	r := &PeLoginResult{UserID: out.Entity.EntityID, Token: out.Entity.Token, Sead: out.Entity.Sead}
	r.Nickname, r.Avatar = peUserDetail(r.UserID, r.Token)
	return r, nil
}

func randomGUID() string {
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(randIntn(256))
	}
	b[6] = (b[6] & 0x0F) | 0x40
	b[8] = (b[8] & 0x3F) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func peUserDetail(uid, token string) (string, string) {
	body := `{"entity_id":"` + uid + `","is_need_recharge_benefit" : 1}`
	path := "/user-detail/query/other"
	req, err := http.NewRequest("POST", "https://g79apigatewayobt.minecraft.cn"+path, strings.NewReader(body))
	if err != nil {
		return "", ""
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("user-agent", "libhttpclient/1.0.0.0")
	req.Header.Set("user-id", uid)
	req.Header.Set("user-token", ComputeDynamicToken(path, body, token))
	req.Header.Set("x_trace_id", randTraceID())
	resp, err := peCoreHTTP.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var o struct {
		Entity struct {
			Nickname  string `json:"nickname"`
			HeadImage string `json:"headImage"`
		} `json:"entity"`
	}
	if json.Unmarshal(raw, &o) != nil {
		return "", ""
	}
	return o.Entity.Nickname, o.Entity.HeadImage
}

func randTraceID() string {
	const cs = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	for i := range b {
		b[i] = cs[randIntn(len(cs))]
	}
	return string(b)
}
