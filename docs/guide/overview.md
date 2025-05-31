---
title: Overview
createTime: 2025/04/01 00:08:53
permalink: /guide/overview
---

## What is StaticPages?

**StaticPages** is a lightweight, self-hosted platform for securely hosting static websites. Designed for developers who want full control and visibility, it provides seamless production and preview deployments through native GitHub Actions integration and S3-compatible object storage.

With a strong focus on performance, observability, and simplicity, StaticPages offers a modern publishing pipeline that's perfect for documentation sites, marketing pages, or any static content.

## Key Features

- 🔐 **Secure by Default** – Authenticate uploads using GitHub OIDC tokens, with fine-grained access control per repository.
- ⚡ **Fast Parallel Uploads** – Files are uploaded concurrently for efficient CI/CD pipelines.
- 🌐 **Custom Domains & Previews** – Serve your site from your own domain, with automatic subdomains for branches or commits.
- 🔁 **CI-Friendly GitHub Action** – Deploy via a single step in your GitHub Actions workflow.
- 📊 **OpenTelemetry Observability** – Track every request with full tracing support using OpenTelemetry.
- ☸️ **Kubernetes-Ready** – Helm Chart available for quick and scalable production deployments.

## Architecture

StaticPages operates as a stateless backend that connects:

1. **GitHub Repositories** – for secure CI/CD uploads via OIDC tokens.
2. **S3-Compatible Object Storage** – for storing static files.
3. **Custom Domains** – to serve content under your own DNS entries.
4. **A Proxy Layer** – for seamless fallback behavior and preview support.

The system supports preview environments out of the box. These are accessible using structured subdomains like:

- `<commit-sha>.your-domain.tld`
- `<branch-name>.your-domain.tld`

## Use Cases

- Project documentation sites
- Preview builds for every PR or branch
- Static sites for microservices and developer portals

For a step-by-step guide to get started, continue to the [Quick Start Guide](/guide/quickstart).
