// SPDX-License-Identifier: MIT
// Copyright 2026 Gregor Jasny

package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"

	storagehelper "github.com/ccache/ccache-go-storage-helper"
)

type storageClient struct {
	client    objectstorage.ObjectStorageClient
	namespace string
	bucket    string
	prefix    string
	logger    *storagehelper.Logger
}

func newStorageClient(cfg *config, logger *storagehelper.Logger) (*storageClient, error) {
	provider, err := newConfigurationProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("could not create OCI configuration provider: %w", err)
	}

	client, err := objectstorage.NewObjectStorageClientWithConfigurationProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("could not create OCI object storage client: %w", err)
	}
	if cfg.Region != "" {
		client.SetRegion(cfg.Region)
	}

	return &storageClient{
		client:    client,
		namespace: cfg.Namespace,
		bucket:    cfg.Bucket,
		prefix:    cfg.Prefix,
		logger:    logger,
	}, nil
}

func newConfigurationProvider(cfg *config) (common.ConfigurationProvider, error) {
	switch cfg.AuthType {
	case authTypeInstancePrincipal:
		return auth.InstancePrincipalConfigurationProvider()
	case authTypeResourcePrincipal:
		return auth.ResourcePrincipalConfigurationProvider()
	case authTypeOkeWorkloadIdentity:
		return auth.OkeWorkloadIdentityConfigurationProvider()
	default: // authTypeAPIKey
		configFile := cfg.ConfigFile
		if configFile == "" {
			configFile = defaultOCIConfigFilePath()
		}
		return common.ConfigurationProviderFromFileWithProfile(configFile, cfg.Profile, "")
	}
}

func defaultOCIConfigFilePath() string {
	if path := os.Getenv("OCI_CONFIG_FILE"); path != "" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".oci", "config")
}

func (s *storageClient) objectName(key []byte) string {
	keyHex := hex.EncodeToString(key)
	if s.prefix == "" {
		return keyHex
	}
	return s.prefix + "/" + keyHex
}

func (s *storageClient) Exists(key []byte) (bool, error) {
	objectName := s.objectName(key)
	s.logger.Logf("EXISTS %s/%s/%s", s.namespace, s.bucket, objectName)

	_, err := s.client.HeadObject(context.Background(), objectstorage.HeadObjectRequest{
		NamespaceName: &s.namespace,
		BucketName:    &s.bucket,
		ObjectName:    &objectName,
	})
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (s *storageClient) Get(key []byte) (io.ReadCloser, int64, bool, error) {
	objectName := s.objectName(key)
	s.logger.Logf("GET %s/%s/%s", s.namespace, s.bucket, objectName)

	resp, err := s.client.GetObject(context.Background(), objectstorage.GetObjectRequest{
		NamespaceName: &s.namespace,
		BucketName:    &s.bucket,
		ObjectName:    &objectName,
	})
	if err != nil {
		if isNotFound(err) {
			return nil, 0, false, nil
		}
		return nil, 0, false, err
	}

	size := int64(-1)
	if resp.ContentLength != nil {
		size = *resp.ContentLength
	}

	return resp.Content, size, true, nil
}

func (s *storageClient) Put(key []byte, value io.Reader, size int64, overwrite bool) (bool, error) {
	objectName := s.objectName(key)

	if !overwrite {
		exists, err := s.Exists(key)
		if err != nil {
			return false, err
		}
		if exists {
			return false, nil
		}
	}

	s.logger.Logf("PUT %s/%s/%s (%d bytes)", s.namespace, s.bucket, objectName, size)

	request := objectstorage.PutObjectRequest{
		NamespaceName: &s.namespace,
		BucketName:    &s.bucket,
		ObjectName:    &objectName,
		ContentLength: &size,
		PutObjectBody: io.NopCloser(value),
	}

	_, err := s.client.PutObject(context.Background(), request)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (s *storageClient) Remove(key []byte) (bool, error) {
	objectName := s.objectName(key)
	s.logger.Logf("REMOVE %s/%s/%s", s.namespace, s.bucket, objectName)

	_, err := s.client.DeleteObject(context.Background(), objectstorage.DeleteObjectRequest{
		NamespaceName: &s.namespace,
		BucketName:    &s.bucket,
		ObjectName:    &objectName,
	})
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func isNotFound(err error) bool {
	serviceErr, ok := common.IsServiceError(err)
	return ok && serviceErr.GetHTTPStatusCode() == 404
}
