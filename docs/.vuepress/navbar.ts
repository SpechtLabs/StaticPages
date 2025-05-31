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

  // {
  //   text: 'Configuration',
  //   items: [
  //     { text: 'Server', link: '/config/server' },
  //     { text: 'Calendars', link: '/config/calendars' },
  //     { text: 'Rules Engine', link: '/config/rules' },
  //     { text: 'Home Assistant Add-On', link: '/config/home_assistant' },
  //   ],
  // },

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
