import { viteBundler } from '@vuepress/bundler-vite';
import { registerComponentsPlugin } from '@vuepress/plugin-register-components';
import { path } from '@vuepress/utils';
import container from 'markdown-it-container';
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

  extendsMarkdown: (md) => {
    md.use(container, 'terminal', {
      validate: (params: string) => /^terminal(?:\s+.*)?$/.test(params.trim()),
      render: (tokens: any[], idx: number) => {
        const token = tokens[idx];

        if (token.nesting === 1) {
          const info = token.info.trim();
          const rest = info.replace(/^terminal\s*/, '');

          const attrs: Record<string, string> = {};
          const attrRegex = /(\w+)=((?:"[^"]*")|(?:'[^']*')|(?:[^\s]+))/g;
          let consumed = '';
          let match: RegExpExecArray | null;

          while ((match = attrRegex.exec(rest)) !== null) {
            const key = match[1];
            let value = match[2];

            if (
              (value.startsWith('"') && value.endsWith('"')) ||
              (value.startsWith("'") && value.endsWith("'"))
            ) {
              value = value.slice(1, -1);
            }

            attrs[key] = value;
            consumed += `${match[0]} `;
          }

          const positional = rest.replace(consumed, '').trim();
          const titleRaw = attrs.title ?? positional ?? '';
          const title = titleRaw ? md.utils.escapeHtml(titleRaw) : '';

          return `\n<Terminal${title ? ` title="${title}"` : ''}>\n`;
        }

        return '\n</Terminal>\n';
      },
    });
  },

  plugins: [
    registerComponentsPlugin({
      componentsDir: path.resolve(__dirname, './components'),
    }),
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
