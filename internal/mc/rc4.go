package mc

type RC4 struct {
	S   [256]byte
	i   byte
	j   byte
	key []byte
}

func NewRC4(key []byte) *RC4 {
	r := &RC4{key: append([]byte(nil), key...)}
	for i := 0; i < 256; i++ {
		r.S[i] = byte(i)
	}
	j := 0
	kl := len(r.key)
	for i := 0; i < 256; i++ {
		j = (j + int(r.S[i]) + int(r.key[i%kl])) % 256
		r.S[i], r.S[j] = r.S[j], r.S[i]
	}
	return r
}

func (r *RC4) Crypt(data []byte) []byte {
	out := make([]byte, len(data))
	for n, b := range data {
		r.i = byte((int(r.i) + 1) % 256)
		r.j = byte((int(r.j) + int(r.S[r.i])) % 256)
		r.S[r.i], r.S[r.j] = r.S[r.j], r.S[r.i]
		k := r.S[(int(r.S[r.i])+int(r.S[r.j]))%256]
		out[n] = b ^ k
	}
	return out
}

func RC4Decrypt(key, data []byte) []byte {
	return NewRC4(key).Crypt(data)
}
