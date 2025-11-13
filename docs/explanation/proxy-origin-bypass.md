---
title: Understanding Proxy Origin Bypass
createTime: 2025/11/12 23:11:54
permalink: /explanation/proxy-origin-bypass/
---

This article explains how StaticPages avoids CloudFlare Error 1000 (DNS resolution loop) when both your StaticPages deployment and storage backend are behind CloudFlare.

## The Problem

When StaticPages runs behind CloudFlare and needs to fetch content from another CloudFlare-proxied domain, a DNS resolution loop can occur.

### Request Flow That Causes the Issue

```text
Browser → CloudFlare DNS → K8s Cluster → StaticPages
                                          ↓
                                     Local DNS → CloudFlare → Error 1000
```

1. Browser resolves your domain via CloudFlare DNS
2. Request reaches your Kubernetes cluster
3. StaticPages needs to fetch from storage (e.g., CDN)
4. Local DNS resolves the storage domain
5. CloudFlare detects potential loop and returns Error 1000

### Why CloudFlare Returns Error 1000

CloudFlare's loop detection identifies that:

- The request originated from a CloudFlare-proxied connection
- The backend request is also going through CloudFlare
- This could create an infinite proxy loop

## How StaticPages Solves This

StaticPages automatically bypasses the issue using **external DNS resolution with direct IP connections**.

### The Solution Flow

```text
Browser → CloudFlare DNS → K8s Cluster → StaticPages
                                          ↓
                                    External DNS (8.8.8.8 / 1.1.1.1)
                                          ↓
                                    Direct IP Connection → Origin Server
```

### Implementation Details

StaticPages uses a custom DNS resolver that:

1. **Queries External DNS Servers**
   - Primary: Google DNS (8.8.8.8)
   - Fallback: Cloudflare DNS (1.1.1.1)
   - Bypasses local DNS entirely

2. **Caches Resolved IPs**
   - Thread-safe caching per hostname
   - Reduces repeated DNS lookups
   - Improves performance

3. **Connects Directly to IPs**
   - Uses resolved IP instead of hostname
   - Maintains correct Host header for virtual hosting
   - Bypasses CloudFlare's loop detection

4. **Falls Back Gracefully**
   - If external DNS fails, uses default DNS
   - Ensures service continuity

## Why This Works

### DNS vs HTTP Layer

CloudFlare's loop detection operates at the DNS/routing layer:

- It sees the source IP making the request
- It checks if that IP belongs to CloudFlare's network
- If yes, and the destination is also CloudFlare, it may block

By resolving DNS externally and connecting directly to the origin IP:

- Local DNS is not used
- CloudFlare doesn't see the recursive resolution
- The HTTP connection goes directly to the origin
- Host header ensures correct virtual host handling

### Host Header Importance

Even though StaticPages connects to an IP address, it sets the correct Host header:

```http
GET /file/bucket/path HTTP/1.1
Host: cdn.example.com
```

This ensures:

- Virtual hosting works correctly
- SSL/TLS certificates validate properly
- The backend serves the correct content

## Configuration

No configuration is required. This feature is **automatic** for all proxy requests.

Your existing configuration works as-is:

```yaml
proxy:
  url: https://cdn.example.com  # Can be CloudFlare-proxied
  path: file/my-bucket
```

StaticPages will:

- Detect it's a proxy request
- Use external DNS resolution
- Connect directly to the origin IP
- Set the correct Host header

## Performance Characteristics

### First Request

- Additional 50-200ms for DNS lookup via external servers
- One-time per hostname until cache expires
- Minimal impact on user experience

### Subsequent Requests

- Uses cached IP (no additional latency)
- Same performance as direct connections
- Benefits from connection pooling

### DNS Cache Behavior

- IPs cached indefinitely during runtime
- New pods perform fresh DNS lookups
- No stale IP issues in dynamic environments

## Observability

### Enable Debug Logging

To see the origin resolution in action:

:::: terminal Enable debug logging

```bash
export LOG_LEVEL=debug
```

::::

### Log Messages to Look For

**Successful Resolution:**

```json
{
  "level": "info",
  "msg": "resolved origin IP",
  "hostname": "cdn.example.com",
  "ip": "104.21.80.230"
}
```

**Using Cached IP:**

```json
{
  "level": "debug",
  "msg": "resolved origin IP via external DNS",
  "host": "cdn.example.com",
  "origin_ip": "104.21.80.230"
}
```

**Fallback to Default DNS:**

```json
{
  "level": "warn",
  "msg": "failed to resolve origin IP via external DNS, using default DNS",
  "host": "cdn.example.com"
}
```

## Troubleshooting

### Still Getting CloudFlare Error 1000?

If you still see errors after deploying StaticPages:

1. **Verify External DNS Access**
   ::: terminal Verify external DNS access

   ```bash
   # From within your cluster
   nslookup google.com 8.8.8.8
   ```

   :::

   If this fails, your cluster may block external DNS.

2. **Check Network Policies**
   ::: terminal Inspect network policies

   ```bash
   kubectl get networkpolicies -A
   ```

   :::

   Ensure policies allow UDP port 53 to 8.8.8.8 and 1.1.1.1.

3. **Review Logs**
   ::: terminal Tail StaticPages logs

   ```bash
   kubectl logs -n static-pages deployment/staticpages | grep "resolved origin IP"
   ```

   :::

   You should see successful resolution messages.

4. **Test DNS Resolution**
   ::: terminal Test DNS resolution from pod

   ```bash
   # Exec into the pod
   kubectl exec -it -n static-pages deployment/staticpages -- sh

   # Try resolving via external DNS
   nslookup cdn.example.com 8.8.8.8
   ```

   :::

### Network Policy Example

If your cluster blocks external DNS, create a policy to allow it:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-external-dns
  namespace: static-pages
spec:
  podSelector:
    matchLabels:
      app: staticpages
  policyTypes:
    - Egress
  egress:
    - to:
        - podSelector: {}
    - to:
        - namespaceSelector: {}
    - ports:
        - protocol: UDP
          port: 53
      to:
        - ipBlock:
            cidr: 8.8.8.8/32  # Google DNS
        - ipBlock:
            cidr: 1.1.1.1/32  # Cloudflare DNS
```

## Comparison with Other Solutions

### Alternative Approach: Bypass CloudFlare

Some solutions disable CloudFlare proxy for the storage backend:

**Pros:**

- Simpler DNS resolution
- No external DNS dependency

**Cons:**

- Loses CloudFlare caching
- Loses CloudFlare DDoS protection
- Exposes origin IP directly

### StaticPages Approach

**Pros:**

- Keeps CloudFlare proxy enabled
- Benefits from CloudFlare caching
- Maintains DDoS protection
- Automatic, no configuration needed

**Cons:**

- Requires external DNS access
- Slight latency on first request

## Related Documentation

- [Backblaze B2 Configuration Reference](/reference/backblaze-b2-config/) - Complete configuration options
- [Fix Backblaze B2 Redirect Issues](/how-to/fix-backblaze-redirect/) - Troubleshooting storage access
