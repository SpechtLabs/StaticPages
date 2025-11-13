import { defineNavbarConfig } from 'vuepress-theme-plume';

export const navbar = defineNavbarConfig([
  { text: 'Home', link: '/', icon: 'mdi:home' },

  {
    text: 'Getting Started',
    icon: 'mdi:rocket-launch',
    items: [
      { text: 'Overview', link: '/guide/overview', icon: 'mdi:eye' },
      { text: 'Quickstart', link: '/guide/quickstart', icon: 'mdi:flash' },
    ],
  },

  {
    text: 'How-to Guides',
    icon: 'mdi:compass',
    items: [
      { text: 'Setup Cloudflare CDN', link: '/how-to/setup-cloudflare-cdn', icon: 'mdi:cloud-outline' },
      { text: 'Fix Backblaze Redirect Issue', link: '/how-to/fix-backblaze-redirect-issue', icon: 'mdi:wrench' },
    ],
  },

  {
    text: 'Explanations',
    icon: 'mdi:lightbulb-outline',
    items: [
      { text: 'Backblaze B2 URL Structure', link: '/explanation/backblaze-b2-url-structure', icon: 'mdi:link-variant' },
      { text: 'Proxy Origin Bypass', link: '/explanation/proxy-origin-bypass', icon: 'mdi:shield-lock-outline' },
    ],
  },

  {
    text: 'Reference',
    icon: 'mdi:book-open-page-variant',
    items: [
      { text: 'Backblaze B2 Config', link: '/reference/backblaze-b2-config', icon: 'mdi:file-cog' },
      { text: 'Helm Chart', link: '/references/helm-chart', icon: 'mdi:kubernetes' },
      { text: 'GitHub Action', link: '/references/action', icon: 'mdi:github' },
    ],
  },

  {
    text: 'More',
    icon: 'mdi:dots-horizontal',
    items: [
      {
        text: 'Download',
        link: 'https://github.com/SpechtLabs/StaticPages/releases',
        target: '_blank',
        rel: 'noopener noreferrer',
        icon: 'mdi:download',
      },
      {
        text: 'Report an Issue',
        link: 'https://github.com/SpechtLabs/StaticPages/issues/new/choose',
        target: '_blank',
        rel: 'noopener noreferrer',
        icon: 'mdi:bug-outline',
      },
    ],
  },
]);
