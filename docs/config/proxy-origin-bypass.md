# Proxy Origin Bypass

When your StaticPages deployment is behind CloudFlare and needs to fetch content from another CloudFlare-proxied domain, you may encounter CloudFlare Error 1000 (DNS resolution loop).

**Good news:** StaticPages automatically handles this for you! No configuration needed.

## The Problem

When a request flows like this:

- Browser → CloudFlare DNS (resolves your domain) → K8s cluster → StaticPages
- StaticPages → Local DNS (resolves CDN domain) → CloudFlare proxy → Error 1000

CloudFlare detects a potential loop and returns error 1000.

## The Solution

StaticPages automatically uses external DNS servers (Google DNS 8.8.8.8 and Cloudflare DNS 1.1.1.1) instead of your local DNS when connecting to proxy URLs. This bypasses the CloudFlare loop detection.

**Your existing configuration works as-is:**

```yaml
pages:
  - bucket:
      applicationId: ENV(APPLICATION_ID)
      name: cedi-testing
      region: eu-central-003
      secret: ENV(S3_SECRET)
      url: https://s3.eu-central-003.backblazeb2.com
    domain: tka.specht-labs.de
    git:
      mainBranch: main
      provider: github
      repository: SpechtLabs/tka
    preview:
      branch: true
      enabled: true
    proxy:
      notFound: 404.html
      path: ""
      searchPath:
        - /index.html
        - /index.htm
      url: https://cdn.specht-labs.de  # Will automatically use external DNS
```

## How It Works

1. **External DNS Resolution**: When StaticPages needs to connect to `cdn.specht-labs.de`, it queries external DNS servers (8.8.8.8 or 1.1.1.1) instead of your local DNS
2. **IP Caching**: The resolved IP is cached to avoid repeated DNS lookups
3. **Direct Connection**: StaticPages connects directly to the resolved IP
4. **Correct Headers**: The Host header is still set to `cdn.specht-labs.de`, so the server knows which virtual host to serve
5. **Loop Prevention**: Since we bypass local DNS and connect directly to the IP, CloudFlare doesn't detect a loop

## Performance

- **First request per hostname**: ~50-200ms additional latency for DNS lookup
- **Subsequent requests**: Use cached IP (no additional latency)
- **Fallback**: If external DNS fails, automatically falls back to local DNS
- **Connection pooling**: Maintains persistent connections for better performance

## Troubleshooting

### Enable Debug Logging

To see the origin resolution in action:

```bash
# Set log level to debug
export LOG_LEVEL=debug

# You'll see log messages like:
# - "resolved origin IP via external DNS"
# - "resolved origin IP" with hostname and IP
```

### Still Getting CloudFlare Error 1000?

If you're still seeing errors:

1. **Check your DNS**: Verify the proxy URL resolves correctly

   ```bash
   nslookup cdn.specht-labs.de 8.8.8.8
   ```

2. **Verify external DNS access**: Ensure your cluster can reach external DNS servers (8.8.8.8, 1.1.1.1)

3. **Check logs**: Look for warnings like "failed to resolve origin IP via external DNS"

4. **Network policies**: Some Kubernetes network policies may block external DNS queries

### Bypass Not Working?

If the automatic bypass isn't working, you can verify the code is running by:

1. Checking logs for "resolved origin IP via external DNS" messages
2. Ensuring you're running the latest version with this feature
3. Verifying no network policies block UDP port 53 to external DNS servers

## Technical Details

StaticPages uses a custom `DialContext` function that:

- Intercepts all outgoing connections
- Resolves hostnames using external DNS servers
- Caches results with a thread-safe map
- Falls back to default DNS if external DNS fails
- Maintains proper Host headers for virtual hosting

This happens transparently for all proxy requests without any configuration needed.
