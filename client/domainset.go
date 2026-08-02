package client

import (
	"bytes"
	"sort"
	"sync"
	"sync/atomic"
)

// DomainSet is an immutable, compact set of DNS names after Build/load.
//
// Storage layout (far smaller than map[string]struct{}):
//   - data: all domains concatenated, ASCII-lowercased
//   - off:  start offset of each domain in data (sorted lexicographically)
//
// Domain i is data[off[i]:end] where end is off[i+1] or len(data).
// Safe for concurrent readers once published (never mutate after publish).
type DomainSet struct {
	data []byte
	off  []uint32
}

// DomainCatalog holds per-list DomainSets for enabled lists only (option B).
// Lookup walks lists; with several lists it may search them in parallel.
// Immutable after publish via atomic.Pointer.
type DomainCatalog struct {
	items []domainList
}

type domainList struct {
	tag string
	set *DomainSet
}

func (s *DomainSet) Len() int {
	if s == nil {
		return 0
	}
	return len(s.off)
}

func (s *DomainSet) domain(i int) []byte {
	start := s.off[i]
	var end uint32
	if i+1 < len(s.off) {
		end = s.off[i+1]
	} else {
		end = uint32(len(s.data))
	}
	return s.data[start:end]
}

// Has reports whether name is in the set. name should already be normalized
// (ASCII lower, no trailing dot) for best performance; non-normalized input
// is still handled correctly via normalizeDomainString.
func (s *DomainSet) Has(name string) bool {
	if s == nil || len(s.off) == 0 || name == "" {
		return false
	}
	// Fast path: already normalized lowercase without trailing dot.
	if !needsDomainNormalize(name) {
		return s.hasNormalized(name)
	}
	return s.hasNormalized(normalizeDomainString(name))
}

func (s *DomainSet) hasNormalized(name string) bool {
	n := len(s.off)
	i, j := 0, n
	for i < j {
		h := int(uint(i+j) >> 1)
		c := cmpStringBytes(name, s.domain(h))
		if c > 0 {
			i = h + 1
		} else if c < 0 {
			j = h
		} else {
			return true
		}
	}
	return false
}

// NewCatalog builds a catalog from parallel slices (nil sets are skipped).
func NewCatalog(tags []string, sets []*DomainSet) *DomainCatalog {
	if len(tags) != len(sets) {
		panic("NewCatalog: len(tags) != len(sets)")
	}
	items := make([]domainList, 0, len(sets))
	for i := range sets {
		if sets[i] == nil || sets[i].Len() == 0 {
			continue
		}
		tag := tags[i]
		items = append(items, domainList{tag: tag, set: sets[i]})
	}
	return &DomainCatalog{items: items}
}

func EmptyCatalog() *DomainCatalog {
	return &DomainCatalog{}
}

func (c *DomainCatalog) Len() int {
	if c == nil {
		return 0
	}
	n := 0
	for i := range c.items {
		n += c.items[i].set.Len()
	}
	return n
}

func (c *DomainCatalog) ListCount() int {
	if c == nil {
		return 0
	}
	return len(c.items)
}

// Snapshot returns a tag→set map for reuse across reloads (enabled lists only).
func (c *DomainCatalog) Snapshot() map[string]*DomainSet {
	if c == nil || len(c.items) == 0 {
		return nil
	}
	m := make(map[string]*DomainSet, len(c.items))
	for i := range c.items {
		m[c.items[i].tag] = c.items[i].set
	}
	return m
}

// Has returns whether name is in any list, plus the tag of the first match.
// With 1 list: single binary search. With 2–3: sequential. With 4+: parallel
// searches with early exit (useful when many independent lists are enabled).
func (c *DomainCatalog) Has(name string) (bool, string) {
	if c == nil || len(c.items) == 0 || name == "" {
		return false, ""
	}
	if needsDomainNormalize(name) {
		name = normalizeDomainString(name)
	}
	if name == "" {
		return false, ""
	}

	items := c.items
	switch len(items) {
	case 1:
		if items[0].set.hasNormalized(name) {
			return true, items[0].tag
		}
		return false, ""
	case 2, 3:
		for i := range items {
			if items[i].set.hasNormalized(name) {
				return true, items[i].tag
			}
		}
		return false, ""
	}

	// Parallel walk for larger catalogs.
	var (
		wg     sync.WaitGroup
		found  atomic.Bool
		tagPtr atomic.Value // string
	)
	// Bound fan-out; each worker does pure CPU binary search.
	for i := range items {
		if found.Load() {
			break
		}
		wg.Add(1)
		go func(dl domainList) {
			defer wg.Done()
			if found.Load() {
				return
			}
			if dl.set.hasNormalized(name) {
				if found.CompareAndSwap(false, true) {
					tagPtr.Store(dl.tag)
				}
			}
		}(items[i])
	}
	wg.Wait()
	if !found.Load() {
		return false, ""
	}
	tag, _ := tagPtr.Load().(string)
	return true, tag
}

// domainBuilder accumulates domains into one buffer (no per-domain string headers
// during the scan), then packs a sorted unique DomainSet.
type domainBuilder struct {
	buf  []byte
	// start offsets into buf for each added domain (unsorted until Build)
	off []uint32
}

func newDomainBuilder(capHint int) *domainBuilder {
	if capHint < 0 {
		capHint = 0
	}
	// Assume ~16 bytes average domain for buffer growth.
	b := &domainBuilder{}
	if capHint > 0 {
		b.buf = make([]byte, 0, capHint*16)
		b.off = make([]uint32, 0, capHint)
	}
	return b
}

// addNormalized appends an already-normalized domain (lower ASCII, no trailing dot).
func (b *domainBuilder) addNormalized(dom []byte) {
	if len(dom) == 0 || b == nil {
		return
	}
	start := uint32(len(b.buf))
	b.buf = append(b.buf, dom...)
	b.off = append(b.off, start)
}

// tryAddLine normalizes a raw list line into the builder without an intermediate
// domain allocation. Returns false if the line is not a valid domain entry.
func (b *domainBuilder) tryAddLine(line []byte) bool {
	if b == nil {
		return false
	}
	line = trimASCIISpaceBytes(line)
	if len(line) == 0 || line[0] == '#' {
		return false
	}
	// Hosts-file style: "0.0.0.0 domain.tld" — use last field.
	if hasASCIISpace(line) {
		fields := splitFields(line)
		if len(fields) == 0 {
			return false
		}
		line = fields[len(fields)-1]
	}
	for len(line) > 0 && line[len(line)-1] == '.' {
		line = line[:len(line)-1]
	}
	if len(line) == 0 || !domainHasDot(line) {
		return false
	}
	start := uint32(len(b.buf))
	// Ensure capacity then write lowercased bytes in place.
	b.buf = append(b.buf, line...)
	dst := b.buf[start:]
	for i := 0; i < len(dst); i++ {
		c := dst[i]
		if c >= 'A' && c <= 'Z' {
			dst[i] = c + ('a' - 'A')
		}
	}
	b.off = append(b.off, start)
	return true
}

func indexByte(b []byte, c byte) int {
	for i := 0; i < len(b); i++ {
		if b[i] == c {
			return i
		}
	}
	return -1
}

func hasASCIISpace(b []byte) bool {
	for i := 0; i < len(b); i++ {
		if b[i] == ' ' || b[i] == '\t' {
			return true
		}
	}
	return false
}

// splitFields splits on ASCII whitespace without allocating if single field.
func splitFields(line []byte) [][]byte {
	// Fast path: no spaces after first trim — caller already trimmed ends.
	hasSpace := false
	for i := 0; i < len(line); i++ {
		if line[i] == ' ' || line[i] == '\t' {
			hasSpace = true
			break
		}
	}
	if !hasSpace {
		return [][]byte{line}
	}
	var out [][]byte
	start := -1
	for i := 0; i <= len(line); i++ {
		if i == len(line) || line[i] == ' ' || line[i] == '\t' {
			if start >= 0 {
				out = append(out, line[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	return out
}

func (b *domainBuilder) count() int {
	if b == nil {
		return 0
	}
	return len(b.off)
}

// Build sorts, uniques, and packs into an immutable DomainSet.
// The builder must not be used after Build.
func (b *domainBuilder) Build() *DomainSet {
	if b == nil || len(b.off) == 0 {
		return &DomainSet{}
	}

	idx := make([]int, len(b.off))
	for i := range idx {
		idx[i] = i
	}

	// ends[i] = exclusive end offset of domain i in add order.
	ends := make([]uint32, len(b.off))
	for i := 0; i < len(b.off)-1; i++ {
		ends[i] = b.off[i+1]
	}
	ends[len(b.off)-1] = uint32(len(b.buf))

	sliceAt := func(i int) []byte {
		return b.buf[b.off[i]:ends[i]]
	}

	sort.Slice(idx, func(i, j int) bool {
		return bytes.Compare(sliceAt(idx[i]), sliceAt(idx[j])) < 0
	})

	// Count unique and total packed size.
	nUnique := 0
	total := 0
	var prev []byte
	for _, i := range idx {
		d := sliceAt(i)
		if nUnique > 0 && bytes.Equal(d, prev) {
			continue
		}
		nUnique++
		total += len(d)
		prev = d
	}

	out := &DomainSet{
		data: make([]byte, 0, total),
		off:  make([]uint32, 0, nUnique),
	}
	prev = nil
	for _, i := range idx {
		d := sliceAt(i)
		if prev != nil && bytes.Equal(d, prev) {
			continue
		}
		out.off = append(out.off, uint32(len(out.data)))
		out.data = append(out.data, d...)
		prev = d
	}
	return out
}

// DomainSetFromDomains builds a set from arbitrary domain strings (normalizes).
// Handy for tests and small merges.
func DomainSetFromDomains(domains []string) *DomainSet {
	b := newDomainBuilder(len(domains))
	for _, d := range domains {
		if nd := normalizeDomainString(d); nd != "" && domainHasDotString(nd) {
			b.addNormalized([]byte(nd))
		}
	}
	return b.Build()
}

// MergeDomainSets packs the union of sets into a new DomainSet.
func MergeDomainSets(sets ...*DomainSet) *DomainSet {
	total := 0
	for _, s := range sets {
		if s != nil {
			total += s.Len()
		}
	}
	b := newDomainBuilder(total)
	for _, s := range sets {
		if s == nil {
			continue
		}
		for i := 0; i < s.Len(); i++ {
			b.addNormalized(s.domain(i))
		}
	}
	return b.Build()
}

func cmpStringBytes(s string, b []byte) int {
	n := len(s)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if s[i] != b[i] {
			if s[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(s) < len(b):
		return -1
	case len(s) > len(b):
		return 1
	default:
		return 0
	}
}

func needsDomainNormalize(name string) bool {
	if name == "" {
		return true
	}
	if name[len(name)-1] == '.' {
		return true
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' || c == ' ' || c == '\t' {
			return true
		}
	}
	return false
}

func normalizeDomainString(name string) string {
	name = trimASCIISpace(name)
	if name == "" {
		return ""
	}
	for len(name) > 0 && name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}
	if name == "" {
		return ""
	}
	// ASCII lower only (DNS labels we care about for blocklists).
	needLower := false
	for i := 0; i < len(name); i++ {
		if name[i] >= 'A' && name[i] <= 'Z' {
			needLower = true
			break
		}
	}
	if !needLower {
		return name
	}
	buf := make([]byte, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		buf[i] = c
	}
	return string(buf)
}

func domainHasDot(b []byte) bool {
	return indexByte(b, '.') >= 0
}

func domainHasDotString(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return true
		}
	}
	return false
}

func trimASCIISpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func trimASCIISpaceBytes(b []byte) []byte {
	start, end := 0, len(b)
	for start < end && (b[start] == ' ' || b[start] == '\t' || b[start] == '\r') {
		start++
	}
	for end > start && (b[end-1] == ' ' || b[end-1] == '\t' || b[end-1] == '\r') {
		end--
	}
	return b[start:end]
}

// estimateDomainCapacity guesses entry count from file size (~20 bytes/line avg).
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
