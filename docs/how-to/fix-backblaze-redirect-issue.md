---
title: Fix Backblaze B2 Redirect Issues
createTime: 2025/11/13 00:00:00
permalink: /how-to/fix-backblaze-redirect/
---

Learn how to fix redirect issues when using Backblaze B2 as your storage backend.

## Problem

When accessing your StaticPages site backed by Backblaze B2, you experience:

- Browser redirects to `backblaze.com`
- HTTP 404 errors for valid pages
- Logs showing "no valid path found after testing all options"

## Solution

Add the `/file/` prefix to your `proxy.path` configuration.

### Steps

1. Open your StaticPages configuration file (e.g., `values.yaml` for Helm deployments)
1. Locate your page configuration with Backblaze B2
1. Update the `proxy.path` to include `file/` prefix:

   ```yaml
   pages:
     - domain: your-domain.com
       bucket:
         region: eu-central-003
         url: https://s3.eu-central-003.backblazeb2.com
         name: your-bucket
         applicationId: ENV(APPLICATION_ID)
         secret: ENV(S3_SECRET)

       proxy:
         url: https://cdn.your-domain.com  # or https://f003.backblazeb2.com
         path: file/your-bucket             # Add 'file/' prefix here
         notFound: 404.html
         searchPath:
           - /index.html
           - /index.htm
   ```

1. Apply the configuration:

   :::: terminal Apply updated Backblaze configuration

   ```bash
   # For Helm deployments
   helm upgrade staticpages spechtlabs/staticpages -f values.yaml -n static-pages

   # For direct deployments
   kubectl rollout restart deployment/staticpages -n static-pages
   ```

   ::::

1. Verify the fix by checking your logs:

   :::: terminal Verify proxy logs

   ```bash
   kubectl logs -n static-pages deployment/staticpages | grep "proxying request"
   ```

   ::::

   You should see paths like `/file/your-bucket/...` instead of `/your-bucket/...`

## Why This Works

Backblaze B2 requires all public file URLs to follow this structure:

```text
https://{endpoint}/file/{bucket-name}/{file-path}
                   ^^^^^
                   This prefix is required
```

The `/file/` prefix tells Backblaze B2 to serve a file from a bucket. Without it, Backblaze returns its homepage, causing the redirect.

This requirement applies even when using:

- A CNAME pointing to Backblaze
- A CDN (like CloudFlare) proxying to Backblaze
- Any custom domain that ultimately routes to Backblaze B2

## Verify It's Working

### Check Your Logs

Look for "proxying request" entries with correct paths:

```json
{
  "level": "info",
  "msg": "proxying request",
  "backend_path": "/file/your-bucket/repo/sha/index.html"
}
```

### Test Manually

Verify your files are accessible:

:::: terminal Compare Backblaze responses

```bash
# This should return your file
curl https://f003.backblazeb2.com/file/your-bucket/path/to/file.html

# This will redirect to Backblaze homepage
curl https://f003.backblazeb2.com/your-bucket/path/to/file.html
```

::::

## Related Issues

- If you're still experiencing issues, see [Troubleshoot Path Resolution](/how-to/troubleshoot-path-resolution/)
- For CloudFlare-specific issues, see [Understanding Proxy Origin Bypass](/explanation/proxy-origin-bypass/)
- For general configuration, see [Backblaze B2 Configuration Reference](/reference/backblaze-b2-config/)
