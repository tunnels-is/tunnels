package client

// DomainSet is an immutable set of domain names after construction.
// Safe for concurrent readers once published via atomic.Pointer (never mutate after Store).
// Uses map[string]struct{} instead of xsync.MapOf to avoid concurrent-map table overhead
// for multi-million DNS blocklists that are rebuilt wholesale on reload.
type DomainSet struct {
	m map[string]struct{}
}

func NewDomainSet(capacity int) *DomainSet {
	if capacity < 0 {
		capacity = 0
	}
	return &DomainSet{m: make(map[string]struct{}, capacity)}
}

func (s *DomainSet) Add(domain string) {
	if s == nil || s.m == nil {
		return
	}
	s.m[domain] = struct{}{}
}

func (s *DomainSet) Has(domain string) bool {
	if s == nil {
		return false
	}
	_, ok := s.m[domain]
	return ok
}

func (s *DomainSet) Len() int {
	if s == nil {
		return 0
	}
	return len(s.m)
}

// MergeFrom copies all domains from other into s (used while building only).
func (s *DomainSet) MergeFrom(other *DomainSet) {
	if s == nil || other == nil {
		return
	}
	for d := range other.m {
		s.m[d] = struct{}{}
	}
}

// estimateDomainCapacity guesses map capacity from file size (~20 bytes/line avg).
func estimateDomainCapacity(fileSize int64) int {
	if fileSize <= 0 {
		return 0
	}
	n := int(fileSize / 20)
	if n < 64 {
		return 64
	}
	return n
}
