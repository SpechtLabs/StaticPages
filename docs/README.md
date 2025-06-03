---
pageLayout: home
externalLinkIcon: false

config:
  - type: doc-hero
    hero:
      name: StaticPages
      text: Static hosting made simple, secure, and scalable.
      tagline: StaticPages is a simple server implementation to host your static pages with support for preview URLs.
      image: /logo.png
      actions:
        - text: Get Started →
          link: /guide/overview
          theme: brand
          icon: simple-icons:bookstack
        - text: GitHub Releases →
          link: https://github.com/SpechtLabs/StaticPages/releases
          theme: alt
          icon: simple-icons:github

  - type: features
    title: Why StaticPages?
    description: A powerful and flexible static hosting backend for your sites, with seamless GitHub CI/CD support.
    features:
      - title: GitHub OIDC Authentication
        icon: mdi:shield-key
        details: Secure, fine-grained access via GitHub Actions with native OIDC token verification.

      - title: Fast Parallel Uploads
        icon: mdi:upload-network-outline
        details: Deploy your site in seconds with parallelized file uploads and real-time feedback.

      - title: Production & Preview Builds
        icon: mdi:web-plus
        details: Easily publish to your domain, with optional preview builds on per-branch or per-commit basis.

      - title: Custom Domains & Wildcard Subdomains
        icon: mdi:domain
        details: Serve sites from your own domains with support for wildcard TLS and subdomain routing.

      - title: S3-Compatible Storage
        icon: mdi:cloud-sync-outline
        details: Use any S3 backend, including Backblaze B2, AWS S3, or MinIO for artifact storage.

      - title: Helm Deployment
        icon: mdi:kubernetes
        details: Ready-to-run Helm chart makes deploying on Kubernetes effortless and secure.

      - title: OpenTelemetry Instrumentation
        icon: mdi:chart-bell-curve-cumulative
        details: Full support for OpenTelemetry tracing for visibility into uploads and page requests.

      - title: Minimal Footprint
        icon: mdi:server
        details: Lightweight Go binary runs anywhere — from Raspberry Pi to production Kubernetes clusters.

  - type: VPReleasesCustom
    repo: SpechtLabs/StaticPages

  - type: VPContributorsCustom
    repo: SpechtLabs/StaticPages
---
