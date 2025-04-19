import { viteBundler } from '@vuepress/bundler-vite'
import { defaultTheme } from '@vuepress/theme-default'
import { path } from '@vuepress/utils'
import { defineUserConfig, PageHeader } from 'vuepress'

import { registerComponentsPlugin } from '@vuepress/plugin-register-components'

function htmlDecode(input: string): string {
  return input.replace("&#39;", "'").replace("&amp;", "&").replace("&quot;", '"')
}

function fixPageHeader(header: PageHeader) {
  header.title = htmlDecode(header.title)
  header.children.forEach(child => fixPageHeader(child))
}

export default defineUserConfig({
  lang: 'en-US',
  title: 'Static Pages',
  description: 'StaticPages is a simple server implementation to host your static pages with support for preview URLs.',

  head: [
    ['meta', { name: "description", content: "StaticPages is a simple server implementation to host your static pages with support for preview URLs." }],
    ['link', { rel: 'icon', href: '/favicon.ico' }]
  ],

  bundler: viteBundler(),

  extendsPage(page, app) {
    const fixedHeaders = page.headers || []
    fixedHeaders.forEach(header => fixPageHeader(header))
  },

  theme: defaultTheme({
    logo: '/logo.png',

    repo: "SpechtLabs/StaticPages",
    docsRepo: "SpechtLabs/StaticPages",
    docsDir: 'docs',
    navbar: [
      {
        text: "Getting Started",
        link: "/guide/README.md",
        children: [
          'guide/README.md',
          'guide/commands.md',
          'guide/home_assistant.md'
        ]
      },
      {
        text: "Configuration",
        link: '/config/',
        children: [
          '/config/calendars.md',
          '/config/server.md',
          '/config/rules.md',
        ]
      },
      {
        text: "Download",
        link: "https://github.com/SpechtLabs/StaticPages/releases",
        target: "_blank"
      },
      {
        text: "Report an Issue",
        link: "https://github.com/SpechtLabs/StaticPages/issues/new/choose",
        target: "_blank"
      }
    ],

    sidebar: {
      '/config/': [
        {
          text: "Configuration",
          children: [
            '/config/calendars.md',
            '/config/server.md',
            '/config/rules.md',
          ]
        }
      ],
      '/guide/': [
        {
          text: "Getting Started",
          children: [
            '/guide/README.md',
            '/guide/commands.md',
            '/guide/home_assistant.md'
          ],
        }
      ],
    }
  }),

  plugins: [
    registerComponentsPlugin({
      componentsDir: path.resolve(__dirname, './components'),
    })
  ]
})
