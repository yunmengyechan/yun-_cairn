package mc

type Event struct {
	Name string
	Data map[string]any
}

type TextMsg struct {
	Type    byte
	Source  string
	Message string
	Extra   string
	SysMsg  string
}

func ParseText(pkt []byte) (*TextMsg, error) {
	_, n := ReadVarint(pkt, 0)
	off := int(n)

	t, off, err := rdU8(pkt, off)
	if err != nil {
		return nil, err
	}
	m := &TextMsg{Type: t}

	switch t {
	case 1:
		if _, off, err = rdU8(pkt, off); err != nil {
			return m, nil
		}
		if m.Source, off, err = rdStringUTF(pkt, off); err != nil {
			return m, nil
		}
		if m.Message, off, err = rdStringUTF(pkt, off); err != nil {
			return m, nil
		}
		if _, off, err = rdI32(pkt, off); err != nil {
			return m, nil
		}
		m.SysMsg, _, _ = rdStringUTF(pkt, off)

	case 9:
		if _, off, err = rdU8(pkt, off); err != nil {
			return m, nil
		}
		if m.Message, off, err = rdStringUTF(pkt, off); err != nil {
			return m, nil
		}
		if raw, _, e := rdByteSlice(pkt, off); e == nil {
			m.Extra = string(raw)
		}

	default:
		if off < len(pkt) {
			off++
		}
		if m.Message, off, err = rdStringUTF(pkt, off); err != nil {
			return m, nil
		}
		m.Extra, _, _ = rdStringUTF(pkt, off)
	}
	return m, nil
}

type PlayerDiff struct {
	Joined []PlayerEntry
	Left   []PlayerEntry
	Total  int
}

type PlayerTracker struct {
	byUUID map[[16]byte]PlayerEntry
}

func NewPlayerTracker() *PlayerTracker {
	return &PlayerTracker{byUUID: make(map[[16]byte]PlayerEntry)}
}

func (t *PlayerTracker) Count() int { return len(t.byUUID) }

func (t *PlayerTracker) All() []PlayerEntry {
	out := make([]PlayerEntry, 0, len(t.byUUID))
	for _, e := range t.byUUID {
		out = append(out, e)
	}
	return out
}

func (t *PlayerTracker) Reset() { t.byUUID = make(map[[16]byte]PlayerEntry) }

func (t *PlayerTracker) Apply(pl *PlayerList) PlayerDiff {
	var d PlayerDiff
	if pl == nil {
		d.Total = len(t.byUUID)
		return d
	}

	if pl.IsAdd() {
		for _, e := range pl.Entries {
			if _, existed := t.byUUID[e.UUID]; !existed {
				d.Joined = append(d.Joined, e)
			}
			t.byUUID[e.UUID] = e
		}
	} else {
		for _, e := range pl.Entries {
			old, existed := t.byUUID[e.UUID]
			if existed {
				delete(t.byUUID, e.UUID)
				old.UUID = e.UUID
				d.Left = append(d.Left, old)
			}
		}
	}
	d.Total = len(t.byUUID)
	return d
}
