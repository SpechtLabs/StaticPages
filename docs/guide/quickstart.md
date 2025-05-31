---
title: Quick Start Guide
permalink: /guide/quickstart
createTime: 2025/06/30 18:19:00
---

Follow this guide to get StaticPages up and running in your Kubernetes cluster and deploy your first static site using GitHub Actions.

## 1. Deploy StaticPages with Helm

First, deploy the StaticPages backend to your existing Kubernetes cluster:

```bash
helm repo add spechtlabs https://charts.specht-labs.de
helm install staticpages spechtlabs/staticpages \
  -n static-pages \
  --create-namespace \
  -f my-values.yaml
```

For full setup instructions, refer to the [Helm Chart documentation](https://github.com/SpechtLabs/StaticPages/tree/main/charts/staticpages) and the [GitHub Action documentation](https://github.com/SpechtLabs/StaticPages-Upload).

## 2. Configure Your Static Page

Update your `my-values.yaml` file with your desired static page configuration. Below is an example using Backblaze B2 as the S3-compatible storage backend:

```yaml
configs:
  pages:
    - domain: specht-labs.de
      bucket:
        region: eu-central-003
        url: https://s3.eu-central-003.backblazeb2.com
        name: static-pages
        applicationId: ENV(APPLICATION_ID)
        secret: ENV(S3_SECRET)

      proxy:
        url: https://f003.backblazeb2.com
        path: file/static-pages
        notFound: 404.html
        searchPath:
          - /index.html
          - /index.htm

      git:
        provider: github
        repository: SpechtLabs/spechtlabs.github.io
        mainBranch: main

      preview:
        enabled: true
        branch: true
        sha: false
        environment: false
```

::: tip
Environment variables like `ENV(APPLICATION_ID)` and `ENV(S3_SECRET)` should be defined via Kubernetes secrets and loaded through Helm's `extraEnvFrom` configuration.
:::

## 3. Set Up GitHub Action for Deployment

Use the following step in your GitHub Actions workflow to deploy your static site:

```yaml
- name: Upload to Static Pages
  uses: SpechtLabs/StaticPages-Upload@main
  with:
    endpoint: https://pages.specht-labs.de
    site-dir: dist/
```

::: note
Ensure `site-dir` points to the directory containing your built static site (e.g., `public/` for Hugo or `dist/` for VuePress).
:::

For a full working example, see the [SpechtLabs Website Deployment Workflow](https://github.com/SpechtLabs/spechtlabs.github.io/blob/main/.github/workflows/deploy.yml#L96).
