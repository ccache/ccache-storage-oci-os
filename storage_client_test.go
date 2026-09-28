// SPDX-License-Identifier: MIT
// Copyright 2026 Gregor Jasny

package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"

	storagehelper "github.com/ccache/ccache-go-storage-helper"
)

func TestObjectName(t *testing.T) {
	sc := &storageClient{prefix: "ccache"}
	if got, want := sc.objectName([]byte{0xab, 0xcd}), "ccache/abcd"; got != want {
		t.Errorf("objectName = %q, want %q", got, want)
	}

	sc.prefix = ""
	if got, want := sc.objectName([]byte{0xab, 0xcd}), "abcd"; got != want {
		t.Errorf("objectName with no prefix = %q, want %q", got, want)
	}
}

// fakeBucket is a minimal in-memory OCI Object Storage backend used to
// exercise storageClient's request construction and response handling
// end-to-end over real HTTP, without talking to a real OCI tenancy.
type fakeBucket struct {
	mu           sync.Mutex
	objects      map[string][]byte
	lastPutCount int
}

func newFakeBucket() *fakeBucket {
	return &fakeBucket{objects: make(map[string][]byte)}
}

func (b *fakeBucket) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	const prefix = "/n/test-namespace/b/test-bucket/o/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}
	objectName := strings.TrimPrefix(r.URL.Path, prefix)

	b.mu.Lock()
	defer b.mu.Unlock()

	switch r.Method {
	case http.MethodHead, http.MethodGet:
		value, ok := b.objects[objectName]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(value)))
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			w.Write(value)
		}

	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		b.objects[objectName] = body
		b.lastPutCount++
		w.WriteHeader(http.StatusOK)

	case http.MethodDelete:
		if _, ok := b.objects[objectName]; !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		delete(b.objects, objectName)
		w.WriteHeader(http.StatusOK)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// newTestStorageClient builds a storageClient backed by an OCI SDK client
// pointed at a local httptest server instead of a real OCI endpoint. The
// throwaway RSA key only needs to be well-formed enough for the SDK's
// request signer to run; the fake server never validates signatures.
func newTestStorageClient(t *testing.T, bucket *fakeBucket) *storageClient {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	provider := common.NewRawConfigurationProvider(
		"ocid1.tenancy.oc1..aaaaaaaatest",
		"ocid1.user.oc1..aaaaaaaatest",
		"us-ashburn-1",
		"aa:bb:cc:dd",
		string(pemKey),
		nil,
	)

	ociClient, err := objectstorage.NewObjectStorageClientWithConfigurationProvider(provider)
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(bucket)
	t.Cleanup(server.Close)
	ociClient.Host = server.URL

	return &storageClient{
		client:    ociClient,
		namespace: "test-namespace",
		bucket:    "test-bucket",
		logger:    storagehelper.NewLogger(""),
	}
}

func TestStorageClientRoundTrip(t *testing.T) {
	bucket := newFakeBucket()
	sc := newTestStorageClient(t, bucket)
	key := []byte{0x01, 0x02, 0x03}

	if exists, err := sc.Exists(key); err != nil || exists {
		t.Fatalf("Exists before Put = (%v, %v), want (false, nil)", exists, err)
	}

	if _, _, found, err := sc.Get(key); err != nil || found {
		t.Fatalf("Get before Put = (found=%v, %v), want (false, nil)", found, err)
	}

	stored, err := sc.Put(key, bytes.NewReader([]byte("payload")), 7, true)
	if err != nil || !stored {
		t.Fatalf("Put = (%v, %v), want (true, nil)", stored, err)
	}

	if exists, err := sc.Exists(key); err != nil || !exists {
		t.Fatalf("Exists after Put = (%v, %v), want (true, nil)", exists, err)
	}

	body, size, found, err := sc.Get(key)
	if err != nil || !found {
		t.Fatalf("Get after Put = (found=%v, %v), want (true, nil)", found, err)
	}
	defer body.Close()
	if size != 7 {
		t.Errorf("Get size = %d, want 7", size)
	}
	data, _ := io.ReadAll(body)
	if string(data) != "payload" {
		t.Errorf("Get body = %q, want %q", data, "payload")
	}

	removed, err := sc.Remove(key)
	if err != nil || !removed {
		t.Fatalf("Remove = (%v, %v), want (true, nil)", removed, err)
	}

	if removed, err := sc.Remove(key); err != nil || removed {
		t.Fatalf("Remove of missing key = (%v, %v), want (false, nil)", removed, err)
	}
}

func TestStorageClientPutOverwriteFalseSkipsExisting(t *testing.T) {
	bucket := newFakeBucket()
	sc := newTestStorageClient(t, bucket)
	key := []byte{0x42}

	stored, err := sc.Put(key, bytes.NewReader([]byte("first")), 5, false)
	if err != nil || !stored {
		t.Fatalf("first Put = (%v, %v), want (true, nil)", stored, err)
	}

	stored, err = sc.Put(key, bytes.NewReader([]byte("second")), 6, false)
	if err != nil || stored {
		t.Fatalf("second Put (overwrite=false) = (%v, %v), want (false, nil)", stored, err)
	}

	_, _, _, err = sc.Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if bucket.lastPutCount != 1 {
		t.Fatalf("expected exactly 1 PUT to reach the backend, got %d", bucket.lastPutCount)
	}
}
