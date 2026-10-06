import { docsKitPlugin } from '@spechtlabs/docs-kit';
import { viteBundler } from '@vuepress/bundler-vite';
import { defineUserConfig } from 'vuepress';
import { plumeTheme } from 'vuepress-theme-plume';

export default defineUserConfig({
  base: '/',
  lang: 'en-US',
  title: 'StaticPages',
  description: 'StaticPages is a simple server implementation to host your static pages with support for preview URLs.',

  head: [
    [
      'meta',
      {
        name: 'description',
        content:
          'StaticPages is a simple server implementation to host your static pages with support for preview URLs.',
      },
    ],
    ['link', { rel: 'icon', type: 'image/png', href: '/images/specht.png' }],
  ],

  bundler: viteBundler(),
  shouldPrefetch: false,

  plugins: [
    // Shared components, the terminal and cast containers, and the
    // contributors and releases data, fetched from GitHub at build time
    docsKitPlugin({ github: { repos: ['SpechtLabs/StaticPages'] } }),
  ],

  theme: plumeTheme({
    docsRepo: 'https://github.com/SpechtLabs/StaticPages',
    docsDir: 'docs',
    docsBranch: 'main',

    editLink: true,
    lastUpdated: false,
    contributors: false,

    cache: 'filesystem',
    search: { provider: 'local' },

    sidebar: {
      '/guide/': [
        {
          text: 'Getting Started',
          icon: 'mdi:rocket-launch',
          prefix: '/guide/',
          items: [
            { text: 'Overview', link: 'overview', icon: 'mdi:eye' },
            { text: 'Quickstart', link: 'quickstart', icon: 'mdi:flash', badge: '5 min' },
          ],
        },
        {
          text: 'How-to Guides',
          icon: 'mdi:compass',
          prefix: '/how-to/',
          items: [
            { text: 'Set up Cloudflare CDN', link: 'setup-cloudflare-cdn', icon: 'mdi:cloud-outline' },
            { text: 'Fix Backblaze Redirect Issue', link: 'fix-backblaze-redirect-issue', icon: 'mdi:wrench' },
          ],
        },
        {
          text: 'Explanations',
          icon: 'mdi:lightbulb-on-outline',
          prefix: '/explanation/',
          items: [
            { text: 'Backblaze B2 URL Structure', link: 'backblaze-b2-url-structure', icon: 'mdi:link-variant' },
            { text: 'Proxy Origin Bypass', link: 'proxy-origin-bypass', icon: 'mdi:shield-lock-outline' },
          ],
        },
        {
          text: 'Reference',
          icon: 'mdi:book-open-page-variant',
          prefix: '/reference/',
          collapsed: false,
          items: [{ text: 'Backblaze B2 Config', link: 'backblaze-b2-config', icon: 'mdi:file-cog' }],
        },
      ],

      '/how-to/': [
        {
          text: 'Getting Started',
          icon: 'mdi:rocket-launch',
          prefix: '/guide/',
          items: [
            { text: 'Overview', link: 'overview', icon: 'mdi:eye' },
            { text: 'Quickstart', link: 'quickstart', icon: 'mdi:flash', badge: '5 min' },
          ],
        },
        {
          text: 'How-to Guides',
          icon: 'mdi:compass',
          prefix: '/how-to/',
          items: [
            { text: 'Set up Cloudflare CDN', link: 'setup-cloudflare-cdn', icon: 'mdi:cloud-outline' },
            { text: 'Fix Backblaze Redirect Issue', link: 'fix-backblaze-redirect-issue', icon: 'mdi:wrench' },
          ],
        },
        {
          text: 'Explanations',
          icon: 'mdi:lightbulb-on-outline',
          prefix: '/explanation/',
          items: [
            { text: 'Backblaze B2 URL Structure', link: 'backblaze-b2-url-structure', icon: 'mdi:link-variant' },
            { text: 'Proxy Origin Bypass', link: 'proxy-origin-bypass', icon: 'mdi:shield-lock-outline' },
          ],
        },
        {
          text: 'Reference',
          icon: 'mdi:book-open-page-variant',
          prefix: '/reference/',
          collapsed: false,
          items: [{ text: 'Backblaze B2 Config', link: 'backblaze-b2-config', icon: 'mdi:file-cog' }],
        },
      ],

      '/explanation/': [
  {
          text: 'Getting Started',
          icon: 'mdi:rocket-launch',
          prefix: '/guide/',
          items: [
            { text: 'Overview', link: 'overview', icon: 'mdi:eye' },
            { text: 'Quickstart', link: 'quickstart', icon: 'mdi:flash', badge: '5 min' },
          ],
        },
        {
          text: 'How-to Guides',
          icon: 'mdi:compass',
          prefix: '/how-to/',
          items: [
            { text: 'Set up Cloudflare CDN', link: 'setup-cloudflare-cdn', icon: 'mdi:cloud-outline' },
            { text: 'Fix Backblaze Redirect Issue', link: 'fix-backblaze-redirect-issue', icon: 'mdi:wrench' },
          ],
        },
        {
          text: 'Explanations',
          icon: 'mdi:lightbulb-on-outline',
          prefix: '/explanation/',
          items: [
            { text: 'Backblaze B2 URL Structure', link: 'backblaze-b2-url-structure', icon: 'mdi:link-variant' },
            { text: 'Proxy Origin Bypass', link: 'proxy-origin-bypass', icon: 'mdi:shield-lock-outline' },
          ],
        },
        {
          text: 'Reference',
          icon: 'mdi:book-open-page-variant',
          prefix: '/reference/',
          collapsed: false,
          items: [{ text: 'Backblaze B2 Config', link: 'backblaze-b2-config', icon: 'mdi:file-cog' }],
        },
      ],

      '/reference/': [
{
          text: 'Getting Started',
          icon: 'mdi:rocket-launch',
          prefix: '/guide/',
          items: [
            { text: 'Overview', link: 'overview', icon: 'mdi:eye' },
            { text: 'Quickstart', link: 'quickstart', icon: 'mdi:flash', badge: '5 min' },
          ],
        },
        {
          text: 'How-to Guides',
          icon: 'mdi:compass',
          prefix: '/how-to/',
          items: [
            { text: 'Set up Cloudflare CDN', link: 'setup-cloudflare-cdn', icon: 'mdi:cloud-outline' },
            { text: 'Fix Backblaze Redirect Issue', link: 'fix-backblaze-redirect-issue', icon: 'mdi:wrench' },
          ],
        },
        {
          text: 'Explanations',
          icon: 'mdi:lightbulb-on-outline',
          prefix: '/explanation/',
          items: [
            { text: 'Backblaze B2 URL Structure', link: 'backblaze-b2-url-structure', icon: 'mdi:link-variant' },
            { text: 'Proxy Origin Bypass', link: 'proxy-origin-bypass', icon: 'mdi:shield-lock-outline' },
          ],
        },
        {
          text: 'Reference',
          icon: 'mdi:book-open-page-variant',
          prefix: '/reference/',
          collapsed: false,
          items: [{ text: 'Backblaze B2 Config', link: 'backblaze-b2-config', icon: 'mdi:file-cog' }],
        },
      ],
    },

    /**
     * markdown
     * @see https://theme-plume.vuejs.press/config/markdown/
     */
    markdown: {
      collapse: true,
      timeline: true,
      plot: true,
      repl: {
        go: true,
        rust: true,
      },
      mermaid: true,
      image: {
        figure: true,
        lazyload: true,
        mark: true,
        size: true,
      },
    },

    watermark: false,
  }),
});
