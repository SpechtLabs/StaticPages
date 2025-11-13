---
title: Setup CloudFlare CDN for StaticPages
createTime: 2025/11/13 00:00:00
permalink: /how-to/setup-cloudflare-cdn/
---

Learn how to configure CloudFlare as a CDN in front of your storage backend to improve performance and reduce bandwidth costs.

## Why Use CloudFlare CDN

Using CloudFlare as a CDN between StaticPages and your storage backend provides:

- **Caching:** Static assets are cached at CloudFlare's edge locations
- **Bandwidth savings:** Reduces requests to your storage backend
- **DDoS protection:** CloudFlare protects your origin from attacks
- **Global performance:** Content served from locations near your users
- **SSL/TLS:** Free SSL certificates for your CDN domain

## Prerequisites

- A domain managed in CloudFlare DNS
- Access to CloudFlare DNS settings
- Your storage backend hostname (e.g., `f003.backblazeb2.com`)
- StaticPages deployed and configured

## Steps

### 1. Create a CNAME Record in CloudFlare

1. Log in to your CloudFlare dashboard
2. Select your domain
3. Go to **DNS** > **Records**
4. Click **Add record**
5. Configure the CNAME:
   - **Type:** CNAME
   - **Name:** Your subdomain (e.g., `cdn` for `cdn.example.com`)
   - **Target:** Your storage backend hostname (e.g., `f003.backblazeb2.com`)
   - **Proxy status:** Enable (orange cloud icon)
   - **TTL:** Auto

6. Click **Save**

### 2. Enable CloudFlare Proxy

The proxy must be enabled (orange cloud icon) for CDN functionality:

- **Enabled (Proxied):** Orange cloud icon - Traffic goes through CloudFlare
- **DNS only:** Gray cloud icon - Direct connection, no caching

Make sure the orange cloud is active.

### 3. Configure Cache Settings (Optional)

For better performance, adjust CloudFlare cache settings:

1. Go to **Caching** > **Configuration**
2. Set **Browser Cache TTL:** 4 hours or higher
3. Go to **Rules** > **Page Rules**
4. Create a page rule for your CDN subdomain:
   - **URL pattern:** `cdn.example.com/*`
   - **Settings:**
     - Cache Level: Cache Everything
     - Edge Cache TTL: 1 month
     - Browser Cache TTL: 4 hours

5. Click **Save and Deploy**

### 4. Update StaticPages Configuration

Update your StaticPages configuration to use the CloudFlare CDN:

```yaml
pages:
  - domain: your-site.com
    bucket:
      region: eu-central-003
      url: https://s3.eu-central-003.backblazeb2.com
      name: my-bucket
      applicationId: ENV(APPLICATION_ID)
      secret: ENV(S3_SECRET)

    proxy:
      url: https://cdn.example.com  # Your CloudFlare CNAME
      path: file/my-bucket          # Include file/ for Backblaze B2
      notFound: 404.html
      searchPath:
        - /index.html
        - /index.htm
```

### 5. Apply the Configuration

Deploy the updated configuration:

:::: terminal Apply updated configuration

```bash
# For Helm deployments
helm upgrade staticpages spechtlabs/staticpages -f values.yaml -n static-pages

# For direct deployments
kubectl apply -f staticpages-config.yaml
kubectl rollout restart deployment/staticpages -n static-pages
```

::::

### 6. Verify the Setup

Test that CloudFlare is caching your content:

:::: terminal Verify CloudFlare caching

```bash
# Make a request and check headers
curl -I https://your-site.com/

# Look for CloudFlare headers
# cf-cache-status: HIT (cached) or MISS (not cached yet)
# cf-ray: CloudFlare ray ID
# server: cloudflare
```

::::

On the first request, you'll see `cf-cache-status: MISS`. On subsequent requests, it should show `HIT`.

## Configuration Examples

### Backblaze B2 with CloudFlare CDN

```yaml
proxy:
  url: https://cdn.example.com
  path: file/my-bucket
```

CNAME setup:

- **Name:** `cdn`
- **Target:** `f003.backblazeb2.com`
- **Proxy:** Enabled (orange cloud)

### Amazon S3 with CloudFlare CDN

```yaml
proxy:
  url: https://cdn.example.com
  path: my-bucket
```

CNAME setup:

- **Name:** `cdn`
- **Target:** `my-bucket.s3.amazonaws.com`
- **Proxy:** Enabled (orange cloud)

### MinIO with CloudFlare CDN

```yaml
proxy:
  url: https://cdn.example.com
  path: my-bucket
```

CNAME setup:

- **Name:** `cdn`
- **Target:** `minio.your-domain.com`
- **Proxy:** Enabled (orange cloud)

## Troubleshooting

### CloudFlare Error 1000 (DNS Resolution Loop)

If you see "Error 1000: DNS points to prohibited IP", don't worry! StaticPages automatically handles this using external DNS resolution.

See: [Understanding Proxy Origin Bypass](/explanation/proxy-origin-bypass/)

### Files Not Caching

If `cf-cache-status` always shows `MISS`:

1. **Check Cache Level:** Set to "Cache Everything" in page rules
2. **Verify Proxy Status:** Orange cloud must be enabled

3. **Check Cache-Control Headers:** Your storage backend might be sending no-cache headers

To override backend headers, add a page rule:

- **Setting:** Cache Level = Cache Everything
- This caches regardless of origin headers

### SSL/TLS Certificate Errors

If you get SSL certificate errors:

1. Go to **SSL/TLS** > **Overview**
2. Set encryption mode to **Full** or **Full (strict)**
3. Wait a few minutes for propagation

### Redirects to Storage Provider

If you're redirected to your storage provider's website:

- For Backblaze B2: Ensure `proxy.path` includes `file/` prefix
- See: [Fix Backblaze B2 Redirect Issues](/how-to/fix-backblaze-redirect/)

## Cache Purging

To clear CloudFlare cache after deploying new content:

### Purge Everything

:::: terminal Purge entire cache

```bash
# Using CloudFlare API
curl -X POST "https://api.cloudflare.com/client/v4/zones/{zone_id}/purge_cache" \
  -H "Authorization: Bearer {api_token}" \
  -H "Content-Type: application/json" \
  --data '{"purge_everything":true}'
```

::::

### Purge Specific Files

:::: terminal Purge specific files

```bash
curl -X POST "https://api.cloudflare.com/client/v4/zones/{zone_id}/purge_cache" \
  -H "Authorization: Bearer {api_token}" \
  -H "Content-Type: application/json" \
  --data '{"files":["https://cdn.example.com/file/bucket/path/file.html"]}'
```

::::

### Via Dashboard

1. Go to **Caching** > **Configuration**
2. Click **Purge Everything** or **Custom Purge**
3. Enter specific URLs if needed
4. Click **Purge**

## Performance Optimization

### Recommended CloudFlare Settings

**Caching:**

- Browser Cache TTL: 4 hours
- Crawlers: Enabled
- Cache Level: Standard (or Cache Everything with page rule)

**Speed:**

- Auto Minify: Enable CSS, JavaScript, HTML

- Brotli: Enabled
- Early Hints: Enabled
- HTTP/2: Enabled
- HTTP/3 (with QUIC): Enabled

**Network:**

- WebSockets: Enabled (if needed)
- gRPC: Disabled (not needed for static sites)

### Cache Everything Page Rule

For maximum performance:

1. **URL pattern:** `cdn.example.com/*`
2. **Settings:**
   - Cache Level: Cache Everything
   - Edge Cache TTL: 1 month
   - Browser Cache TTL: 4 hours
   - Origin Cache Control: On

## Cost Considerations

CloudFlare Free tier includes:

- Unlimited bandwidth
- Unlimited caching
- 3 page rules
- Basic DDoS protection

This is typically sufficient for StaticPages deployments.

## Security Considerations

### Enable Additional CloudFlare Security

1. **Bot Fight Mode:** Go to **Security** > **Bots**
2. **Challenge Passage:** Go to **Security** > **Settings**
3. **Rate Limiting:** Create rules to prevent abuse

### SSL/TLS Settings

Recommended settings:

- **Encryption mode:** Full (strict)
- **Minimum TLS Version:** TLS 1.2
- **Opportunistic Encryption:** Enabled
- **TLS 1.3:** Enabled
- **Automatic HTTPS Rewrites:** Enabled

## Related Documentation

- [Understanding Proxy Origin Bypass](/explanation/proxy-origin-bypass/) - How StaticPages avoids CloudFlare loops
- [Backblaze B2 Configuration Reference](/reference/backblaze-b2-config/) - Complete B2 configuration
- [Fix Backblaze B2 Redirect Issues](/how-to/fix-backblaze-redirect/) - Troubleshoot redirect problems
