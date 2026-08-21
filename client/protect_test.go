package client

import (
	"fmt"
	"strings"
	"testing"
)

func TestNewProtectBindWrapsWhenIfIndexSet(t *testing.T) {
	plain := newProtectBind(0)
	if _, ok := plain.(*ifaceProtectBind); ok {
		t.Fatal("ifIndex 0 should not wrap the default bind")
	}
	wrapped := newProtectBind(7)
	b, ok := wrapped.(*ifaceProtectBind)
	if !ok {
		t.Fatal("ifIndex > 0 should wrap the bind")
	}
	if b.ifIndex != 7 {
		t.Fatalf("ifIndex = %d, want 7", b.ifIndex)
	}
}

func TestWGIPCConfigIncludesFwMarkAndEndpoint(t *testing.T) {
	s := wgIPCConfig("aa", "bb", "74.63.223.157", "51820")
	wantMark := fmt.Sprintf("fwmark=%d", wgProtectMark)
	if !strings.Contains(s, wantMark) {
		t.Fatalf("fwmark missing from UAPI config:\n%s", s)
	}
	if !strings.Contains(s, "endpoint=74.63.223.157:51820") {
		t.Fatalf("endpoint missing from UAPI config:\n%s", s)
	}
	if !strings.Contains(s, "allowed_ip=0.0.0.0/0") {
		t.Fatalf("allowed_ip missing from UAPI config:\n%s", s)
	}
}
