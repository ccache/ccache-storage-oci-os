# ccache-storage-oci-os

A [ccache remote storage helper](https://ccache.dev/storage-helpers.html) for
[OCI Object Storage](https://www.oracle.com/cloud/storage/object-storage/),
written in **Go** using the official [OCI Go
SDK](https://github.com/oracle/oci-go-sdk).

## Overview

This is a storage helper for [ccache] that enables caching compilation results
in OCI Object Storage buckets. It implements the [ccache remote storage helper
protocol].

See also [ccache-storage-http-go] for a similar helper for generic HTTP(S)
storage servers, which this project follows the structure of.

[ccache]: https://ccache.dev
[ccache remote storage helper protocol]: https://github.com/ccache/ccache/blob/master/doc/remote_storage_helper_spec.md
[ccache-storage-http-go]: https://github.com/ccache/ccache-storage-http-go

## Features

- Stores cache entries as objects in an OCI Object Storage bucket, using the
  official OCI Go SDK.
- Multiple authentication mechanisms: API key (config file/profile),
  instance principal, resource principal, and OKE workload identity.
- Cross-platform: Linux, macOS, Windows.
- Optional debug logging.

## Installation

The helper should be installed in a [location where ccache searches for helper
programs] under the name `ccache-storage-oci-os`.

[location where ccache searches for helper programs]: https://github.com/ccache/ccache/blob/master/doc/manual.adoc#storage-helper-process

### Using a prebuilt binary

Grab a prebuilt binary from
[Releases](https://github.com/ccache/ccache-storage-oci-os/releases) and place
it in a suitable directory as described above.

### Building from source

```bash
# Clone the repository:
git clone https://github.com/ccache/ccache-storage-oci-os
cd ccache-storage-oci-os

# On Windows:
go mod download
go build -ldflags="-s -w" -trimpath -o ccache-storage-oci-os.exe .

# On Linux/macOS and similar:
make

# Install ccache-storage-oci-os in /usr/local/bin:
make install

# Install ccache-storage-oci-os in /example/dir:
make install INSTALL_DIR=/example/dir
```

## Configuration

The helper is configured via ccache's [`remote_storage` configuration]. The
binary is automatically invoked by ccache when needed.

The remote storage URL has the form `oci-os://<bucket>[/<prefix>]`, where
`<bucket>` is the name of an existing OCI Object Storage bucket and the
optional `<prefix>` is a key prefix ("directory") within that bucket.

For example:

```bash
# Set the CCACHE_REMOTE_STORAGE environment variable:
export CCACHE_REMOTE_STORAGE="oci-os://my-bucket/ccache @namespace=axaz1abcdefg"

# Or set remote_storage in ccache's configuration file:
ccache -o remote_storage="oci-os://my-bucket/ccache @namespace=axaz1abcdefg"
```

[`remote_storage` configuration]: https://github.com/ccache/ccache/blob/master/doc/manual.adoc#remote-storage-backends

### Configuration attributes

- `@namespace` (**required**): The OCI Object Storage namespace of the
  tenancy that owns the bucket.
- `@region`: OCI region identifier (e.g. `eu-frankfurt-1`). If not set, the
  region is derived from the selected authentication mechanism (e.g. the
  config file profile's `region` setting, or the instance/resource
  principal's region).
- `@auth-type`: Authentication mechanism, one of:
  - `api_key` (default): Use an OCI config file, as used by the OCI CLI.
  - `instance_principal`: Authenticate as the OCI Compute instance the helper
    runs on.
  - `resource_principal`: Authenticate as the OCI resource (e.g. Functions)
    the helper runs in.
  - `oke_workload_identity`: Authenticate using OKE workload identity.
- `@config-file`: Path to the OCI config file (only used with
  `@auth-type=api_key`). Defaults to the `OCI_CONFIG_FILE` environment
  variable if set, otherwise `~/.oci/config`.
- `@profile`: OCI config file profile name (only used with
  `@auth-type=api_key`). Defaults to `DEFAULT`.

### Authentication examples

```bash
# API key, default profile in ~/.oci/config:
export CCACHE_REMOTE_STORAGE="oci-os://my-bucket/ccache @namespace=axaz1abcdefg"

# API key, named profile:
export CCACHE_REMOTE_STORAGE="oci-os://my-bucket/ccache @namespace=axaz1abcdefg @profile=CCACHE"

# Instance principal (running on an OCI Compute instance):
export CCACHE_REMOTE_STORAGE="oci-os://my-bucket/ccache @namespace=axaz1abcdefg @auth-type=instance_principal"

# OKE workload identity:
export CCACHE_REMOTE_STORAGE="oci-os://my-bucket/ccache @namespace=axaz1abcdefg @auth-type=oke_workload_identity"
```

## Optional debug logging

You can set the `CRSH_LOGFILE` environment variable to enable debug logging to a
file:

```bash
export CRSH_LOGFILE=/path/to/debug.log
```

Note: The helper process is spawned by ccache, so the environment variable must
be set before ccache is invoked.

Warning: The debug log is not redacted and may contain secrets such as
tenancy/bucket names and namespace identifiers. Only enable it for
troubleshooting and protect or delete the log file afterwards.

## Contributing

Contributions are welcome! Please submit pull requests or open issues on GitHub.
