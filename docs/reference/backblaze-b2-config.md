---
title: Backblaze B2 Configuration Reference
createTime: 2025/11/13 00:00:00
permalink: /reference/backblaze-b2-config/
---

Complete configuration reference for using Backblaze B2 with StaticPages.

## Basic Configuration

### Direct Backblaze Connection

```yaml
pages:
  - domain: example.com
    bucket:
      region: eu-central-003
      url: https://s3.eu-central-003.backblazeb2.com
      name: my-bucket
      applicationId: ENV(APPLICATION_ID)
      secret: ENV(S3_SECRET)

    proxy:
      url: https://f003.backblazeb2.com
      path: file/my-bucket
      notFound: 404.html
      searchPath:
        - /index.html
        - /index.htm

    git:
      provider: github
      repository: org/repo
      mainBranch: main

    preview:
      enabled: true
      branch: true
      sha: false
      environment: false
```

### With CDN/CloudFlare

```yaml
pages:
  - domain: example.com
    bucket:
      region: eu-central-003
      url: https://s3.eu-central-003.backblazeb2.com
      name: my-bucket
      applicationId: ENV(APPLICATION_ID)
      secret: ENV(S3_SECRET)

    proxy:
      url: https://cdn.example.com  # CNAME to f003.backblazeb2.com
      path: file/my-bucket           # Still requires file/ prefix
      notFound: 404.html
      searchPath:
        - /index.html
        - /index.htm
```

## Configuration Fields

### `bucket` Section

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `region` | string | Yes | Backblaze B2 region identifier (e.g., `eu-central-003`) |
| `url` | string | Yes | S3-compatible API endpoint URL |
| `name` | string | Yes | Bucket name |
| `applicationId` | string | Yes | B2 Application Key ID (use `ENV(VAR_NAME)` for secrets) |
| `secret` | string | Yes | B2 Application Key (use `ENV(VAR_NAME)` for secrets) |

### `proxy` Section

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `url` | string | Yes | Public file access URL (Backblaze endpoint or CDN) |
| `path` | string | Yes | Must be `file/{bucket-name}` for Backblaze B2 |
| `notFound` | string | No | Path to 404 page (default: `404.html`) |
| `searchPath` | array | No | Paths to try when direct path fails (e.g., `["/index.html"]`) |

## Region Endpoints

Backblaze B2 regions and their corresponding endpoints:

| Region | S3 API Endpoint | Public Download Endpoint |
| --- | --- | --- |
| `us-west-001` | `s3.us-west-001.backblazeb2.com` | `f001.backblazeb2.com` |
| `us-west-002` | `s3.us-west-002.backblazeb2.com` | `f002.backblazeb2.com` |
| `eu-central-003` | `s3.eu-central-003.backblazeb2.com` | `f003.backblazeb2.com` |
| `us-west-004` | `s3.us-west-004.backblazeb2.com` | `f004.backblazeb2.com` |
| `us-east-005` | `s3.us-east-005.backblazeb2.com` | `f005.backblazeb2.com` |

## URL Construction Examples

Given this configuration:

```yaml
proxy:
  url: https://f003.backblazeb2.com
  path: file/my-bucket
```

StaticPages constructs URLs like:

| Original Request | Resolved Path | Full URL |
| --- | --- | --- |
| `/` | `/file/my-bucket/org/repo/sha/index.html` | `https://f003.backblazeb2.com/file/my-bucket/org/repo/sha/index.html` |
| `/about/` | `/file/my-bucket/org/repo/sha/about/index.html` | `https://f003.backblazeb2.com/file/my-bucket/org/repo/sha/about/index.html` |
| `/style.css` | `/file/my-bucket/org/repo/sha/style.css` | `https://f003.backblazeb2.com/file/my-bucket/org/repo/sha/style.css` |

## Preview Configuration

With preview enabled:

```yaml
preview:
  enabled: true
  branch: true    # Access via branch-name.example.com
  sha: true       # Access via sha.example.com
  environment: false
```

Preview URLs are constructed as:

- Branch: `https://f003.backblazeb2.com/file/my-bucket/org/repo/{branch-sha}/path`
- SHA: `https://f003.backblazeb2.com/file/my-bucket/org/repo/{sha}/path`
- Main: `https://f003.backblazeb2.com/file/my-bucket/org/repo/{main-sha}/path`

## Environment Variables

For secure credential management:

```yaml
bucket:
  applicationId: ENV(B2_APPLICATION_ID)
  secret: ENV(B2_APPLICATION_KEY)
```

Set these in your Helm values:

```yaml
extraEnvFrom:
  - secretRef:
      name: b2-credentials
```

Create the secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: b2-credentials
type: Opaque
stringData:
  B2_APPLICATION_ID: "your-app-id"
  B2_APPLICATION_KEY: "your-app-key"
```

## Multiple Sites Configuration

```yaml
pages:
  - domain: site1.example.com
    bucket:
      name: bucket1
      # ...
    proxy:
      path: file/bucket1
      # ...

  - domain: site2.example.com
    bucket:
      name: bucket2
      # ...
    proxy:
      path: file/bucket2
      # ...
```

## Comparison with Other Providers

### Amazon S3

```yaml
proxy:
  url: https://s3.amazonaws.com
  path: my-bucket  # No file/ prefix needed
```

### MinIO

```yaml
proxy:
  url: https://minio.example.com
  path: my-bucket  # No file/ prefix needed
```

### Cloudflare R2

```yaml
proxy:
  url: https://pub-xxxxx.r2.dev
  path: ""  # Bucket implicit in domain
```

## Related Documentation

- [Fix Backblaze B2 Redirect Issues](/how-to/fix-backblaze-redirect/) - Troubleshooting guide
- [Understanding Backblaze B2 URL Structure](/explanation/backblaze-b2-url-structure/) - Why the `/file/` prefix is required
- [Proxy Origin Bypass](/explanation/proxy-origin-bypass/) - CloudFlare-specific considerations
