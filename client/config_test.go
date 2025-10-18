package client

import (
	"testing"

	"github.com/tunnels-is/tunnels/certs"
)

func TestDefaultConfig(t *testing.T) {
	conf := DefaultConfig()

	if conf == nil {
		t.Fatal("DefaultConfig should not return nil")
	}

	// Test boolean defaults
	if !conf.DebugLogging {
		t.Error("DebugLogging should be true by default")
	}
	if !conf.InfoLogging {
		t.Error("InfoLogging should be true by default")
	}
	if !conf.ErrorLogging {
		t.Error("ErrorLogging should be true by default")
	}
	if conf.ConnectionTracer {
		t.Error("ConnectionTracer should be false by default")
	}

	// Test DNS defaults
	if conf.DNSServerIP != "127.0.0.1" {
		t.Errorf("DNSServerIP should be 127.0.0.1, got %s", conf.DNSServerIP)
	}
	if conf.DNSServerPort != "53" {
		t.Errorf("DNSServerPort should be 53, got %s", conf.DNSServerPort)
	}
	if conf.DNS1Default != "1.1.1.1" {
		t.Errorf("DNS1Default should be 1.1.1.1, got %s", conf.DNS1Default)
	}
	if conf.DNS2Default != "8.8.8.8" {
		t.Errorf("DNS2Default should be 8.8.8.8, got %s", conf.DNS2Default)
	}

	// Test API defaults
	if conf.APIIP != "127.0.0.1" {
		t.Errorf("APIIP should be 127.0.0.1, got %s", conf.APIIP)
	}
	if conf.APIPort != "7777" {
		t.Errorf("APIPort should be 7777, got %s", conf.APIPort)
	}

	// Test update defaults
	if conf.RestartPostUpdate {
		t.Error("RestartPostUpdate should be false by default")
	}
	if conf.ExitPostUpdate {
		t.Error("ExitPostUpdate should be false by default")
	}
	if !conf.AutoDownloadUpdate {
		t.Error("AutoDownloadUpdate should be true by default")
	}
	if conf.UpdateWhileConnected {
		t.Error("UpdateWhileConnected should be false by default")
	}
	if !conf.DisableUpdates {
		t.Error("DisableUpdates should be true by default")
	}

	// Test logging defaults
	if !conf.LogBlockedDomains {
		t.Error("LogBlockedDomains should be true by default")
	}
	if !conf.LogAllDomains {
		t.Error("LogAllDomains should be true by default")
	}
	if !conf.DNSstats {
		t.Error("DNSstats should be true by default")
	}

	// Test that block/white lists are initialized
	if conf.DNSBlockLists == nil {
		t.Error("DNSBlockLists should not be nil")
	}
	if conf.DNSWhiteLists == nil {
		t.Error("DNSWhiteLists should not be nil")
	}

	// Test control servers
	if len(conf.ControlServers) != 1 {
		t.Errorf("Should have 1 default control server, got %d", len(conf.ControlServers))
	} else {
		cs := conf.ControlServers[0]
		if cs.ID != "tunnels" {
			t.Errorf("Default control server ID should be 'tunnels', got %s", cs.ID)
		}
		if cs.Host != "api.tunnels.is" {
			t.Errorf("Default control server Host should be 'api.tunnels.is', got %s", cs.Host)
		}
		if cs.Port != "443" {
			t.Errorf("Default control server Port should be '443', got %s", cs.Port)
		}
		if !cs.ValidateCertificate {
			t.Error("Default control server should validate certificates")
		}
	}

	// Test certificate defaults
	if conf.APIKey != "./api.key" {
		t.Errorf("APIKey should be './api.key', got %s", conf.APIKey)
	}
	if conf.APICert != "./api.crt" {
		t.Errorf("APICert should be './api.crt', got %s", conf.APICert)
	}
	if conf.APICertType != certs.RSA {
		t.Errorf("APICertType should be RSA, got %v", conf.APICertType)
	}

	// Test certificate IPs
	if len(conf.APICertIPs) != 2 {
		t.Errorf("Should have 2 default cert IPs, got %d", len(conf.APICertIPs))
	} else {
		if conf.APICertIPs[0] != "127.0.0.1" {
			t.Errorf("First cert IP should be 127.0.0.1, got %s", conf.APICertIPs[0])
		}
		if conf.APICertIPs[1] != "0.0.0.0" {
			t.Errorf("Second cert IP should be 0.0.0.0, got %s", conf.APICertIPs[1])
		}
	}

	// Test certificate domains
	if len(conf.APICertDomains) != 2 {
		t.Errorf("Should have 2 default cert domains, got %d", len(conf.APICertDomains))
	} else {
		if conf.APICertDomains[0] != "tunnels.app" {
			t.Errorf("First cert domain should be tunnels.app, got %s", conf.APICertDomains[0])
		}
		if conf.APICertDomains[1] != "app.tunnels.is" {
			t.Errorf("Second cert domain should be app.tunnels.is, got %s", conf.APICertDomains[1])
		}
	}

	t.Logf("Default config validation passed")
}

func TestApplyCertificateDefaultsToConfig(t *testing.T) {
	// Test with empty config
	cfg := &configV2{}
	applyCertificateDefaultsToConfig(cfg)

	if cfg.APIKey != "./api.key" {
		t.Errorf("APIKey should be set to './api.key', got %s", cfg.APIKey)
	}
	if cfg.APICert != "./api.crt" {
		t.Errorf("APICert should be set to './api.crt', got %s", cfg.APICert)
	}
	if cfg.APICertType != certs.RSA {
		t.Errorf("APICertType should be RSA, got %v", cfg.APICertType)
	}
	if len(cfg.APICertIPs) != 2 {
		t.Errorf("Should have 2 cert IPs, got %d", len(cfg.APICertIPs))
	}
	if len(cfg.APICertDomains) != 2 {
		t.Errorf("Should have 2 cert domains, got %d", len(cfg.APICertDomains))
	}

	// Test with existing values (should not override)
	cfg2 := &configV2{
		APIKey:  "/custom/key.pem",
		APICert: "/custom/cert.pem",
	}
	applyCertificateDefaultsToConfig(cfg2)

	if cfg2.APIKey != "/custom/key.pem" {
		t.Errorf("APIKey should not be overridden, got %s", cfg2.APIKey)
	}
	if cfg2.APICert != "/custom/cert.pem" {
		t.Errorf("APICert should not be overridden, got %s", cfg2.APICert)
	}

	// Test with existing cert IPs (should not override)
	cfg3 := &configV2{
		APICertIPs: []string{"192.168.1.1"},
	}
	applyCertificateDefaultsToConfig(cfg3)

	if len(cfg3.APICertIPs) != 1 {
		t.Errorf("APICertIPs should not be overridden, got %d entries", len(cfg3.APICertIPs))
	}
	if cfg3.APICertIPs[0] != "192.168.1.1" {
		t.Errorf("APICertIPs should not be overridden, got %s", cfg3.APICertIPs[0])
	}

	// Test with existing cert domains (should not override)
	cfg4 := &configV2{
		APICertDomains: []string{"custom.domain.com"},
	}
	applyCertificateDefaultsToConfig(cfg4)

	if len(cfg4.APICertDomains) != 1 {
		t.Errorf("APICertDomains should not be overridden, got %d entries", len(cfg4.APICertDomains))
	}
	if cfg4.APICertDomains[0] != "custom.domain.com" {
		t.Errorf("APICertDomains should not be overridden, got %s", cfg4.APICertDomains[0])
	}

	t.Logf("Certificate defaults application test passed")
}
