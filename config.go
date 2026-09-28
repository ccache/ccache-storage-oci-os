// SPDX-License-Identifier: MIT
// Copyright 2026 Gregor Jasny

package main

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	storagehelper "github.com/ccache/ccache-go-storage-helper"
)

type authType string

const (
	authTypeAPIKey              authType = "api_key"
	authTypeInstancePrincipal   authType = "instance_principal"
	authTypeResourcePrincipal   authType = "resource_principal"
	authTypeOkeWorkloadIdentity authType = "oke_workload_identity"
)

type config struct {
	*storagehelper.Config
	Bucket     string
	Prefix     string
	Namespace  string
	Region     string
	AuthType   authType
	ConfigFile string
	Profile    string
}

func parseConfig(logger *storagehelper.Logger) (*config, error) {
	commonConfig, err := storagehelper.ParseConfig(logger)
	if err != nil {
		return nil, err
	}

	cfg := &config{
		Config:   commonConfig,
		AuthType: authTypeAPIKey,
	}

	parsedURL, err := url.Parse(cfg.Config.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid CRSH_URL: %w", err)
	}
	cfg.Bucket = parsedURL.Host
	cfg.Prefix = strings.Trim(parsedURL.Path, "/")
	if cfg.Bucket == "" {
		cfg.Diagnostics = append(cfg.Diagnostics, "error: CRSH_URL must be of the form oci-os://<bucket>[/<prefix>]")
	}
	logger.Logf("Bucket: %s, prefix: %s", cfg.Bucket, cfg.Prefix)

	for _, attribute := range cfg.Attributes {
		key := attribute.Key
		value := attribute.Value

		switch key {
		case "namespace":
			if value == "" {
				cfg.Diagnostics = append(cfg.Diagnostics, "error: namespace cannot be empty")
			}
			cfg.Namespace = value
		case "region":
			if value == "" {
				cfg.Diagnostics = append(cfg.Diagnostics, "error: region cannot be empty")
			}
			cfg.Region = value
		case "auth-type":
			switch authType(value) {
			case authTypeAPIKey, authTypeInstancePrincipal, authTypeResourcePrincipal, authTypeOkeWorkloadIdentity:
				cfg.AuthType = authType(value)
			default:
				cfg.Diagnostics = append(cfg.Diagnostics, fmt.Sprintf("error: unknown auth-type: %s", value))
			}
		case "config-file":
			if value == "" {
				cfg.Diagnostics = append(cfg.Diagnostics, "error: config-file cannot be empty")
			} else {
				// The helper changes its working directory to the filesystem root after
				// configuration, so resolve a relative path to absolute here.
				absPath, err := filepath.Abs(value)
				if err != nil {
					cfg.Diagnostics = append(cfg.Diagnostics, fmt.Sprintf("error: invalid config-file path %q: %v", value, err))
				} else {
					cfg.ConfigFile = absPath
				}
			}
		case "profile":
			if value == "" {
				cfg.Diagnostics = append(cfg.Diagnostics, "error: profile cannot be empty")
			}
			cfg.Profile = value
		default:
			cfg.Diagnostics = append(cfg.Diagnostics, fmt.Sprintf("warning: unknown attribute: %s", key))
		}
	}

	if cfg.Namespace == "" {
		cfg.Diagnostics = append(cfg.Diagnostics, "error: namespace attribute is required")
	}

	if cfg.AuthType != authTypeAPIKey && cfg.ConfigFile != "" {
		cfg.Diagnostics = append(cfg.Diagnostics, "warning: config-file is ignored unless auth-type is api_key")
	}
	if cfg.AuthType != authTypeAPIKey && cfg.Profile != "" {
		cfg.Diagnostics = append(cfg.Diagnostics, "warning: profile is ignored unless auth-type is api_key")
	}
	if cfg.AuthType == authTypeAPIKey && cfg.Profile == "" {
		cfg.Profile = "DEFAULT"
	}

	for _, diag := range cfg.Diagnostics {
		logger.Logf("%s", diag)
	}

	return cfg, nil
}
