---
title: GitHub Action
createTime: 2025/04/01 00:08:53
permalink: /references/action
---

This GitHub Action simplifies deploying static sites (e.g., built with Hugo, VuePress, Astro, etc.) by securely uploading your generated files to a Static Pages backend via a single composite step.

## Features

- Simple integration with any static site generator
- Secure authentication using GitHub OIDC
- Fast, parallel file uploads with detailed summaries
- Upload results available in the GitHub Action summary

## Inputs

| Name       | Description                                                                 | Required | Default |
|------------|-----------------------------------------------------------------------------|----------|---------|
| `endpoint` | **Base URL** of your Static Pages API (e.g. `staticpages.example.com`)      | **yes**  | —       |
| `site-dir` | Path to the directory containing the **generated static site files**        | **yes**  | —       |
| `verbose`  | Enable verbose curl output (`true` or `false`)                              | no       | `false` |

## Output

| Name             | Description                    |
|------------------|--------------------------------|
| `uploaded-count` | The number of files uploaded   |

## Example Usage

```yaml
name: Deploy Static Site

on:
  workflow_dispatch:
  push:
    branches: [main]

permissions:
  id-token: write   # Required for OIDC authentication
  contents: read    # To read repository contents

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4
        with:
          submodules: recursive
          fetch-depth: 0

      - name: Initialize submodules
        run: git submodule update --init --recursive

      - name: Setup Hugo
        uses: peaceiris/actions-hugo@v2
        with:
          hugo-version: "latest"
          extended: true

      - name: Build site
        run: hugo --minify

      - name: Upload to Static Pages
        uses: SpechtLabs/StaticPages-Upload@v1
        with:
          endpoint: https://staticpages.example.com
          site-dir: public/
