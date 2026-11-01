package mc

import (
	"bytes"
	"compress/flate"
	"io"
	"sync"
)

type ZlibStreamCompressor struct {
	mu       sync.Mutex
	w        *flate.Writer
	rb       *bytes.Buffer
	acc      *bytes.Buffer
	produced int
}

func NewZlibStream() (*ZlibStreamCompressor, error) {
	z := &ZlibStreamCompressor{rb: &bytes.Buffer{}, acc: &bytes.Buffer{}}
	w, err := flate.NewWriter(z.rb, flate.DefaultCompression)
	if err != nil {
		return nil, err
	}
	z.w = w
	return z, nil
}

func (z *ZlibStreamCompressor) Compress(data []byte) ([]byte, error) {
	z.mu.Lock()
	defer z.mu.Unlock()
	if z.w == nil {
		w, err := flate.NewWriter(z.rb, flate.DefaultCompression)
		if err != nil {
			return nil, err
		}
		z.w = w
	}
	start := z.rb.Len()
	if _, err := z.w.Write(data); err != nil {
		return nil, err
	}
	if err := z.w.Flush(); err != nil {
		return nil, err
	}
	out := make([]byte, z.rb.Len()-start)
	copy(out, z.rb.Bytes()[start:])
	return out, nil
}

func (z *ZlibStreamCompressor) Decompress(data []byte) ([]byte, error) {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.acc.Write(data)
	r := flate.NewReader(bytes.NewReader(z.acc.Bytes()))
	defer r.Close()
	full, err := io.ReadAll(r)
	if len(full) == 0 && err != nil {
		return nil, err
	}
	if len(full) <= z.produced {
		return nil, nil
	}
	out := make([]byte, len(full)-z.produced)
	copy(out, full[z.produced:])
	z.produced = len(full)
	return out, nil
}

func ZlibCompress(data []byte) ([]byte, error) {
	var out bytes.Buffer
	w, err := flate.NewWriter(&out, flate.DefaultCompression)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func ZlibDecompress(data []byte) ([]byte, error) {
	r := flate.NewReader(bytes.NewReader(data))
	defer r.Close()
	return io.ReadAll(r)
}
