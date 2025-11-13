---
title: Understanding Backblaze B2 URL Structure
createTime: 2025/11/13 00:00:00
permalink: /explanation/backblaze-b2-url-structure/
---

This article explains why Backblaze B2 has specific URL requirements and how they differ from other S3-compatible storage providers.

## Why Backblaze B2 Requires the `/file/` Prefix

Backblaze B2 serves multiple purposes beyond just file storage. A single endpoint handles:

- File downloads (public and private)
- API requests for bucket management
- Authentication endpoints
- Web interface access

To distinguish file requests from other operations, Backblaze requires a specific URL path structure.

## The URL Anatomy

A Backblaze B2 public file URL has this structure:

```text
https://{endpoint}/file/{bucket-name}/{file-path}
         └─────┬────┘ └──┬──┘ └─────┬────┘ └───┬───┘
           endpoint   prefix  bucket    file path
```

### Components

**Endpoint**: The Backblaze B2 region endpoint

- Example: `f003.backblazeb2.com`
- Region-specific (e.g., `eu-central-003` uses `f003`)

**Prefix**: Always `/file/` for public file access

- Tells Backblaze this is a file download request
- Required even for custom domains/CNAMEs
- Cannot be omitted or changed

**Bucket Name**: Your B2 bucket identifier

- Must match exactly
- Case-sensitive

**File Path**: The path to your file within the bucket

- Can include subdirectories
- Relative to bucket root

## How It Differs From Other Providers

### Amazon S3

Amazon S3 uses virtual-hosted-style or path-style URLs:

```text
# Virtual-hosted-style (preferred)
https://bucket-name.s3.amazonaws.com/file-path

# Path-style (legacy)
https://s3.amazonaws.com/bucket-name/file-path
```

No `/file/` prefix is needed because S3 uses dedicated subdomains or distinguishes buckets at the DNS level.

### MinIO

MinIO follows S3 conventions:

```text
https://minio.example.com/bucket-name/file-path
```

### Cloudflare R2

Cloudflare R2 can use custom domains with files at the root:

```text
https://pub-xxxxx.r2.dev/file-path
```

The bucket is implicit in the domain itself.

## Why CNAMEs Don't Change This

When you create a CNAME pointing to Backblaze:

```text
cdn.example.com  →  f003.backblazeb2.com
```

The CNAME only replaces the hostname in DNS resolution. The HTTP request path remains unchanged:

```text
Before: https://f003.backblazeb2.com/file/bucket/path
After:  https://cdn.example.com/file/bucket/path
                                ^^^^^^^^^^^^
                                Path stays the same
```

This is why StaticPages must include the `/file/` prefix in the `proxy.path` configuration, even when using a CDN or CNAME.

## What Happens Without the Prefix

When Backblaze receives a request without `/file/`:

```text
https://f003.backblazeb2.com/bucket-name/file.html
                             ↑
                             Missing /file/ prefix
```

Backblaze interprets this as:

- Not a file download request
- Possibly a web interface access attempt
- Returns the Backblaze homepage with a redirect to `backblaze.com`

This explains the redirect behavior users experience with misconfigured paths.

## Design Philosophy

Backblaze's URL structure reflects its multi-tenant, multi-purpose API design:

1. **Single Endpoint**: One domain handles all operations
2. **Path-Based Routing**: URL path determines operation type
3. **Backward Compatibility**: Structure established early and maintained
4. **Simplicity**: No complex subdomain management required

While different from S3, this approach has advantages:

- No DNS wildcard requirements
- Simpler endpoint configuration
- Clear separation between operations

## Summary

The `/file/` prefix is a fundamental part of Backblaze B2's API design, not a configuration option. When using StaticPages with Backblaze B2:

- Always include `file/bucket-name` in your `proxy.path`
- This applies regardless of CDN, CNAME, or proxy configuration
- Other storage providers (S3, MinIO, R2) have different structures

For configuration examples, see the [Backblaze B2 Configuration Reference](/reference/backblaze-b2-config/).
