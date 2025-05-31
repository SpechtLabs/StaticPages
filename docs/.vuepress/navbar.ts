import { defineNavbarConfig } from 'vuepress-theme-plume'

export const navbar = defineNavbarConfig([
  { text: 'Home', link: '/' },

  {
    text: 'Getting Started',
    items: [
      { text: 'Overview', link: '/guide/overview' },
      { text: 'Quick Start', link: '/guide/quickstart' },
    ],
  },

  {
    text: 'References',
    items: [
      { text: 'Helm-Chart', link: '/references/helm-chart' },
      { text: 'GitHub Action', link: '/references/action' },
    ],
  },

  {
    text: 'Download',
    link: 'https://github.com/SpechtLabs/StaticPages/releases',
    target: '_blank',
    rel: 'noopener noreferrer',
  },

  {
    text: 'Report an Issue',
    link: 'https://github.com/SpechtLabs/StaticPages/issues/new/choose',
    target: '_blank',
    rel: 'noopener noreferrer',
  },
])
