package raknet

import (
	"sync"
)

const maxFragmentPayload = 1340

type Splitter struct {
	mu     sync.Mutex
	nextID uint16
}

func NewSplitter() *Splitter { return &Splitter{} }

func (s *Splitter) Split(body []byte, reliability int) []*Frame {
	if len(body) <= maxFragmentPayload {
		return []*Frame{{Reliability: reliability, Body: body}}
	}
	s.mu.Lock()
	s.nextID++
	id := s.nextID
	s.mu.Unlock()

	var chunks [][]byte
	for i := 0; i < len(body); i += maxFragmentPayload {
		end := i + maxFragmentPayload
		if end > len(body) {
			end = len(body)
		}
		chunks = append(chunks, body[i:end])
	}

	compoundSize := uint32(len(chunks))

	frames := make([]*Frame, 0, len(chunks))
	for i, ch := range chunks {
		frames = append(frames, &Frame{
			Reliability:   reliability,
			Body:          ch,
			Fragmented:    true,
			CompoundSize:  compoundSize,
			CompoundID:    id,
			FragmentIndex: uint32(i),
		})
	}
	return frames
}

type Assembler struct {
	mu    sync.Mutex
	parts map[uint16]*partSet
}

type partSet struct {
	total   uint32
	chunks  map[uint32][]byte
	created int64
}

func NewAssembler() *Assembler {
	return &Assembler{parts: make(map[uint16]*partSet)}
}

func (a *Assembler) Feed(f *Frame) []byte {
	a.mu.Lock()
	defer a.mu.Unlock()
	ps := a.parts[f.CompoundID]
	if ps == nil {
		ps = &partSet{total: f.CompoundSize, chunks: make(map[uint32][]byte)}
		a.parts[f.CompoundID] = ps
	}
	ps.chunks[f.FragmentIndex] = f.Body
	if len(ps.chunks) < int(f.CompoundSize) {
		return nil
	}

	full := []byte{}
	for i := uint32(0); i < f.CompoundSize; i++ {
		c, ok := ps.chunks[i]
		if !ok {
			return nil
		}
		full = append(full, c...)
	}
	delete(a.parts, f.CompoundID)
	return full
}
