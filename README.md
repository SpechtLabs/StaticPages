# StaticPages

[![Release](https://github.com/SpechtLabs/StgaticPages/actions/workflows/release.yaml/badge.svg)](https://github.com/SpechtLabs/StgaticPages/actions/workflows/release.yaml)
[![Continuous Integration](https://github.com/SpechtLabs/StgaticPages/actions/workflows/build.yaml/badge.svg)](https://github.com/SpechtLabs/StgaticPages/actions/workflows/build.yaml)
[![Documentation](https://github.com/SpechtLabs/StgaticPages/actions/workflows/docs-website.yaml/badge.svg)](https://github.com/SpechtLabs/StgaticPages/actions/workflows/docs-website.yaml)

StaticPages is a lightweight, self-hosted service for securely publishing static websites. It supports production and preview deployments with minimal operational overhead and integrates seamlessly with GitHub Actions, S3-compatible storage, and Kubernetes.

## Features

- **Secure by default** – Uses GitHub OIDC tokens for fine-grained access control.
- **Fast deployments** – Uploads are parallelized and optimized for performance.
- **Custom domains & previews** – Host your static site under your own domain, with support for preview builds per branch or commit.
- **Simple GitHub Action integration** – Easily trigger uploads from your CI/CD pipeline.
- **Open Telemetry Instrumentation** – Observability done right with Open Telemetry. Analyze ever single request with traces.
- **Kubernetes-ready** – Helm Chart available for production-grade deployments.

## How It Works

StaticPages hosts static websites from S3-compatible storage, reverse-proxying requests under custom domains. GitHub repositories can upload site artifacts using a GitHub Action, authenticated via OIDC.

Preview builds are automatically made available under subdomains like:

- `<commit>.your-domain.tld`
- `<branch>.your-domain.tld`

## Architecture

StaticPages is a stateless backend that:

- Authenticates upload requests using GitHub OIDC
- Uploads artifacts to S3-compatible storage
- Proxies site access via configurable URLs and fallback strategies
- Serves preview builds via commit- and branch-based subdomains

## Use the GitHub Action

To deploy static generated pages from GitHub Action workflows:

```yaml
- name: Upload to Static Pages
  uses: SpechtLabs/StaticPages-Upload@v1
  with:
    endpoint: https://staticpages.example.com
    site-dir: public/
```

For full setup instructions, see the [`StaticPages-Upload@v1` Action Documentation](https://github.com/SpechtLabs/StaticPages-Upload):

### Deploy with Helm

```bash
helm repo add spechtlabs https://charts.specht-labs.de
helm install staticpages spechtlabs/staticpages -n static-pages --create-namespace -f my-values.yaml
```

For full setup instructions, see the [Helm Chart documentation](https://github.com/SpechtLabs/StaticPages/tree/main/charts/staticpages).

## Contributing

Contributions are welcome! Feel free to open issues or pull requests. For larger changes, please start with a discussion in the issue section.
