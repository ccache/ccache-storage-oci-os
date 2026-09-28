// SPDX-License-Identifier: MIT
// Copyright 2026 Gregor Jasny

package main

import (
	"fmt"
	"strings"
	"testing"

	storagehelper "github.com/ccache/ccache-go-storage-helper"
)

func clearCRSHEnv(t *testing.T) {
	t.Helper()
	names := []string{
		"CRSH_IPC_ENDPOINT",
		"CRSH_URL",
		"CRSH_IDLE_TIMEOUT",
		"CRSH_NUM_ATTR",
	}
	for i := 0; i < 8; i++ {
		names = append(names, fmt.Sprintf("CRSH_ATTR_KEY_%d", i), fmt.Sprintf("CRSH_ATTR_VALUE_%d", i))
	}
	for _, name := range names {
		t.Setenv(name, "")
	}
}

func setAttrs(t *testing.T, attrs map[string]string) {
	t.Helper()
	i := 0
	for key, value := range attrs {
		t.Setenv(fmt.Sprintf("CRSH_ATTR_KEY_%d", i), key)
		t.Setenv(fmt.Sprintf("CRSH_ATTR_VALUE_%d", i), value)
		i++
	}
	t.Setenv("CRSH_NUM_ATTR", fmt.Sprintf("%d", i))
}

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		attrs       map[string]string
		wantDiag    string // substring expected among diagnostics
		wantBucket  string
		wantPrefix  string
		wantNS      string
		wantRegion  string
		wantAuth    authType
		wantProfile string
	}{
		{
			name:     "missing namespace",
			url:      "oci-os://my-bucket",
			wantDiag: "namespace attribute is required",
		},
		{
			name:        "bucket and prefix from URL",
			url:         "oci-os://my-bucket/some/prefix",
			attrs:       map[string]string{"namespace": "ns1"},
			wantBucket:  "my-bucket",
			wantPrefix:  "some/prefix",
			wantNS:      "ns1",
			wantAuth:    authTypeAPIKey,
			wantProfile: "DEFAULT",
		},
		{
			name:     "missing bucket",
			url:      "oci-os:///prefix-only",
			attrs:    map[string]string{"namespace": "ns1"},
			wantDiag: "CRSH_URL must be of the form",
		},
		{
			name:       "region and auth-type",
			url:        "oci-os://my-bucket",
			attrs:      map[string]string{"namespace": "ns1", "region": "eu-frankfurt-1", "auth-type": "instance_principal"},
			wantBucket: "my-bucket",
			wantNS:     "ns1",
			wantRegion: "eu-frankfurt-1",
			wantAuth:   authTypeInstancePrincipal,
		},
		{
			name:     "unknown auth-type",
			url:      "oci-os://my-bucket",
			attrs:    map[string]string{"namespace": "ns1", "auth-type": "bogus"},
			wantDiag: "unknown auth-type: bogus",
		},
		{
			name:     "unknown attribute",
			url:      "oci-os://my-bucket",
			attrs:    map[string]string{"namespace": "ns1", "bogus-attr": "x"},
			wantDiag: "unknown attribute: bogus-attr",
		},
		{
			name:        "custom profile",
			url:         "oci-os://my-bucket",
			attrs:       map[string]string{"namespace": "ns1", "profile": "CCACHE"},
			wantBucket:  "my-bucket",
			wantNS:      "ns1",
			wantAuth:    authTypeAPIKey,
			wantProfile: "CCACHE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearCRSHEnv(t)
			t.Setenv("CRSH_URL", tt.url)
			setAttrs(t, tt.attrs)

			logger := storagehelper.NewLogger("")
			cfg, err := parseConfig(logger)
			if err != nil {
				t.Fatalf("parseConfig failed: %v", err)
			}

			if tt.wantDiag != "" {
				found := false
				for _, diag := range cfg.Diagnostics {
					if strings.Contains(diag, tt.wantDiag) {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("expected diagnostic containing %q, got %v", tt.wantDiag, cfg.Diagnostics)
				}
				return
			}

			if len(cfg.Diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", cfg.Diagnostics)
			}
			if cfg.Bucket != tt.wantBucket {
				t.Errorf("Bucket = %q, want %q", cfg.Bucket, tt.wantBucket)
			}
			if cfg.Prefix != tt.wantPrefix {
				t.Errorf("Prefix = %q, want %q", cfg.Prefix, tt.wantPrefix)
			}
			if cfg.Namespace != tt.wantNS {
				t.Errorf("Namespace = %q, want %q", cfg.Namespace, tt.wantNS)
			}
			if cfg.Region != tt.wantRegion {
				t.Errorf("Region = %q, want %q", cfg.Region, tt.wantRegion)
			}
			if cfg.AuthType != tt.wantAuth {
				t.Errorf("AuthType = %q, want %q", cfg.AuthType, tt.wantAuth)
			}
			if tt.wantProfile != "" && cfg.Profile != tt.wantProfile {
				t.Errorf("Profile = %q, want %q", cfg.Profile, tt.wantProfile)
			}
		})
	}
}
