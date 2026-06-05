import { defineConfig } from 'astro/config';
import sitemap from '@astrojs/sitemap';

export default defineConfig({
  site: 'https://jedipunkz.rocks',
  base: '/ax/',
  output: 'static',
  integrations: [sitemap()],
});
