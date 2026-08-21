package types

import "testing"

func TestValidateWANCIDR(t *testing.T) {
	ok := []string{"", "10.0.0.0/8", "10.20.0.0/16", "172.16.0.0/12", "192.168.0.0/16", "10.0.0.1/32"}
	for _, cidr := range ok {
		if err := ValidateWANCIDR(cidr); err != nil {
			t.Errorf("%q: %v", cidr, err)
		}
	}
	bad := []string{
		"0.0.0.0/0", "0.0.0.0/1", "128.0.0.0/1", "0.0.0.0/7",
		"10.0.0.0/7", "::/0", "fd00::/8", "not-a-cidr",
	}
	for _, cidr := range bad {
		if err := ValidateWANCIDR(cidr); err == nil {
			t.Errorf("%q: accepted, want reject", cidr)
		}
	}
}
