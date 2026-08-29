import { defineConfig } from 'astro/config';
import sitemap from '@astrojs/sitemap';

export default defineConfig({
  site: 'https://jedipunkz.rocks',
  base: '/agx/',
  output: 'static',
  integrations: [sitemap()],
  vite: {
    ssr: {
      external: ['@resvg/resvg-js'],
    },
  },
});
