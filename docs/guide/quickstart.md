---
title: Quick Start Guide
permalink: /guide/quickstart
createTime: 2025/06/30 18:19:00
---

Follow this guide to get StaticPages up and running in your Kubernetes cluster and deploy your first static site using GitHub Actions.

## 1. Deploy StaticPages with Helm

First, deploy the StaticPages backend to your existing Kubernetes cluster:

:::: terminal Deploy StaticPages

```bash
helm repo add spechtlabs https://charts.specht-labs.de
helm install staticpages spechtlabs/staticpages \
  -n static-pages \
  --create-namespace \
  -f my-values.yaml
```

::::

Then, create a secret containing your S3 credentials, typically managed via ksops and kustomize:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: s3-credentials
  namespace: static-pages
type: Opaque
stringData:
  app-id: <your-app-id>
  s3-secret: <your-base64-encoded-s3-secret>
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

### Sharing settings across pages with `pageDefaults`

If you host several sites from the same bucket and CDN, declare the shared
settings once under a top-level `pageDefaults` block. Every entry in `pages` is
deep-merged over it, so each page only needs the fields that differ:

```yaml
configs:
  pageDefaults:
    bucket:
      region: eu-central-003
      url: https://s3.eu-central-003.backblazeb2.com
      name: static-pages
      applicationId: ENV(APPLICATION_ID)
      secret: ENV(S3_SECRET)
    proxy:
      url: https://cdn.specht-labs.de
      path: file/static-pages
      notFound: 404.html
      searchPath: [.html, .htm, /index.html, /index.htm]
    git:
      provider: github
      mainBranch: main
    preview:
      enabled: true
      branch: true

  pages:
    - domain: prose.specht-labs.de
      git: { repository: SpechtLabs/prose }

    - domain: dev.specht-labs.de
      git: { repository: SpechtLabs/tka }
      preview: { enabled: false }   # overrides the default; `branch` stays inherited
```

Merge rules:

- Nested blocks merge per field — a page that sets only `git.repository` keeps the default `git.provider` and `git.mainBranch`.
- A field set on a page always wins, including a zero value: `preview.enabled: false` overrides a `true` default.
- Lists replace rather than append; a page's `searchPath` fully supersedes the default one.
- `domain` is always per page. Omit `pageDefaults` entirely and the config behaves exactly as before.

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
