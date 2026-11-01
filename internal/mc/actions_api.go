package mc

func (e PlayerEntry) UUIDString() string { return uuidString(e.UUID) }

func (s *Session) Players() []PlayerEntry {
	if s.tracker == nil {
		return nil
	}
	return s.tracker.All()
}

func (s *Session) PlayerCount() int {
	if s.tracker == nil {
		return 0
	}
	return s.tracker.Count()
}
