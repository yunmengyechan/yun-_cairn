package mc

import "math"

type Rotor struct {
	rotors    int
	size      int
	sizeMask  int
	seed      [3]int
	key       [5]int
	positions []int
	advances  []int
	eRotor    []int
	dRotor    []int
	isinited  bool
}

const rotorSize = 256

func cDiv(a, b int) int { return a / b }
func cMod(a, b int) int { return a % b }

func short(v int) int {
	v &= 0xFFFF
	if v >= 0x8000 {
		return v - 0x10000
	}
	return v
}

func NewRotor(key []byte, numRotors int) *Rotor {
	r := &Rotor{rotors: numRotors, size: rotorSize, sizeMask: 0}
	r.setKey(key)
	return r
}

func (r *Rotor) setKey(key []byte) {
	k1, k2, k3, k4, k5 := 995, 576, 767, 671, 463
	for _, b := range key {
		ki := int(b) & 0xFF
		k1 = ((k1<<3 | k1>>13) + ki) & 0xFFFF
		k2 = ((k2<<3 | k2>>13) ^ ki) & 0xFFFF
		k3 = ((k3<<3 | k3>>13) - ki) & 0xFFFF
		k4 = (ki - (k4<<3 | k4>>13)) & 0xFFFF
		k5 = ((k5<<3 | k5>>13) ^ (^ki & 0xFFFF)) & 0xFFFF
	}
	r.key[0] = short(k1)
	r.key[1] = short(k2 | 1)
	r.key[2] = short(k3)
	r.key[3] = short(k4)
	r.key[4] = short(k5)
	r.setSeed()
}

func (r *Rotor) setSeed() {
	r.seed[0] = r.key[0]
	r.seed[1] = r.key[1]
	r.seed[2] = r.key[2]
	r.isinited = false
}

func (r *Rotor) rRandom() float64 {
	x, y, z := r.seed[0], r.seed[1], r.seed[2]
	x = 171*cMod(x, 177) - 2*cDiv(x, 177)
	y = 172*cMod(y, 176) - 35*cDiv(y, 176)
	z = 170*cMod(z, 178) - 63*cDiv(z, 178)
	if x < 0 {
		x += 30269
	}
	if y < 0 {
		y += 30307
	}
	if z < 0 {
		z += 30323
	}
	r.seed[0], r.seed[1], r.seed[2] = x, y, z
	term := float64(x)/30269.0 + float64(y)/30307.0 + float64(z)/30323.0
	val := term - math.Floor(term)
	if val >= 1.0 {
		val = 0.0
	}
	return val
}

func (r *Rotor) rRand(s int) int {
	v := int(r.rRandom() * float64(s))
	return v % s
}

func (r *Rotor) permuteRotor(e []int, eoff int, d []int, doff int) {
	for j := 0; j < r.size; j++ {
		d[doff+j] = j
	}
	for j := 0; j < r.size; j++ {
		e[eoff+j] = j
	}
	i := r.size
	for 2 <= i {
		q := r.rRand(i)
		i--
		j := e[eoff+q]
		e[eoff+q] = e[eoff+i]
		e[eoff+i] = j
		d[doff+j] = i
	}
	d[doff+e[eoff+0]] = 0
}

func (r *Rotor) init() {
	r.setSeed()
	n := r.rotors
	r.positions = make([]int, n)
	r.advances = make([]int, n)
	r.eRotor = make([]int, n*r.size)
	r.dRotor = make([]int, n*r.size)
	for i := 0; i < n; i++ {
		r.positions[i] = r.rRand(r.size)
		r.advances[i] = 1 + 2*r.rRand(r.size/2)
		r.permuteRotor(r.eRotor, i*r.size, r.dRotor, i*r.size)
	}
	r.isinited = true
}

func (r *Rotor) advance() {
	for i := 0; i < r.rotors; i++ {
		temp := r.positions[i] + r.advances[i]
		if r.sizeMask != 0 {
			r.positions[i] = temp & r.sizeMask
		} else {
			r.positions[i] = temp % r.size
		}
		if temp >= r.size && i < r.rotors-1 {
			r.positions[i+1] = (1 + r.positions[i+1]) & 0xFF
		}
	}
}

func (r *Rotor) eChar(p int) int {
	tp := p
	for i := 0; i < r.rotors; i++ {
		idx := (r.positions[i] ^ tp) % r.size
		tp = r.eRotor[i*r.size+idx]
	}
	r.advance()
	return tp & 0xFF
}

func (r *Rotor) dChar(c int) int {
	tc := c
	for i := r.rotors - 1; i >= 0; i-- {
		tc = (r.positions[i] ^ r.dRotor[i*r.size+tc]) % r.size
	}
	r.advance()
	return tc & 0xFF
}

func (r *Rotor) Decrypt(data []byte) []byte {
	if !r.isinited {
		r.init()
	}
	out := make([]byte, len(data))
	for i, b := range data {
		out[i] = byte(r.dChar(int(b)))
	}
	return out
}

func (r *Rotor) Encrypt(data []byte) []byte {
	if !r.isinited {
		r.init()
	}
	out := make([]byte, len(data))
	for i, b := range data {
		out[i] = byte(r.eChar(int(b)))
	}
	return out
}
