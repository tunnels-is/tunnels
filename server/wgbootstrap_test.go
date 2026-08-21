package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tunnels-is/tunnels/types"
)

func TestWgBootstrapSkipVerify(t *testing.T) {
	if !wgBootstrapSkipVerify("selfsign") || !wgBootstrapSkipVerify("SelfSign") {
		t.Fatal("selfsign must skip verify")
	}
	for _, v := range []string{"", "example.com", "vpn.example.com"} {
		if wgBootstrapSkipVerify(v) {
			t.Fatalf("%q must verify", v)
		}
	}
}

func TestWriteWGConfig_SkipVerifyFromCertMode(t *testing.T) {
	dir := t.TempDir()
	prev := wgConfigPath
	t.Cleanup(func() { wgConfigPath = prev })
	wgConfigPath = filepath.Join(dir, "wg-config.json")

	if err := writeWGConfig(true); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(wgConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	var got types.WGBootstrap
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if !got.InsecureSkipVerify {
		t.Fatal("selfsign bootstrap must set InsecureSkipVerify")
	}

	// Existing file is not rewritten.
	if err := writeWGConfig(false); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(wgConfigPath)
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if !got.InsecureSkipVerify {
		t.Fatal("existing wg-config must not be overwritten")
	}
}

func TestWriteWGConfig_VerifyByDefault(t *testing.T) {
	dir := t.TempDir()
	prev := wgConfigPath
	t.Cleanup(func() { wgConfigPath = prev })
	wgConfigPath = filepath.Join(dir, "wg-config.json")

	if err := writeWGConfig(false); err != nil {
		t.Fatal(err)
	}
	cfg := WGConfig.Load()
	if cfg == nil || cfg.InsecureSkipVerify {
		t.Fatal("LE / standalone generate must verify certs")
	}
}
