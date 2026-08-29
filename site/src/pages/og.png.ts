import satori from 'satori';
import { Resvg } from '@resvg/resvg-js';
import type { APIRoute } from 'astro';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const fontRegular = readFileSync(
  resolve(process.cwd(), 'node_modules/@fontsource/inter/files/inter-latin-400-normal.woff')
);
const fontBold = readFileSync(
  resolve(process.cwd(), 'node_modules/@fontsource/inter/files/inter-latin-700-normal.woff')
);

export const GET: APIRoute = async () => {
  const svg = await satori(
    {
      type: 'div',
      props: {
        style: {
          display: 'flex',
          flexDirection: 'column',
          width: '1200px',
          height: '630px',
          backgroundColor: '#1e1e2e',
        },
        children: [
          // Top accent bar
          {
            type: 'div',
            props: {
              style: {
                height: '6px',
                background: 'linear-gradient(to right, #89b4fa, #cba6f7)',
                width: '100%',
                flexShrink: 0,
              },
            },
          },
          // Main content area
          {
            type: 'div',
            props: {
              style: {
                display: 'flex',
                flexDirection: 'column',
                flex: 1,
                padding: '60px 80px',
              },
              children: [
                // Top section — fills remaining space
                {
                  type: 'div',
                  props: {
                    style: {
                      display: 'flex',
                      flexDirection: 'column',
                      flex: 1,
                    },
                    children: [
                      {
                        type: 'div',
                        props: {
                          style: {
                            display: 'flex',
                            fontSize: 128,
                            fontWeight: 700,
                            color: '#cdd6f4',
                            lineHeight: 1,
                            letterSpacing: '-4px',
                          },
                          children: 'agx',
                        },
                      },
                      {
                        type: 'div',
                        props: {
                          style: {
                            display: 'flex',
                            fontSize: 22,
                            fontWeight: 400,
                            color: '#a6adc8',
                            marginTop: '32px',
                            lineHeight: 1.6,
                          },
                          children: 'Parallel AI coding agent orchestrator for isolated git worktrees',
                        },
                      },
                    ],
                  },
                },
                // Bottom — repo URL
                {
                  type: 'div',
                  props: {
                    style: {
                      display: 'flex',
                      fontSize: 20,
                      fontWeight: 400,
                      color: '#6c7086',
                    },
                    children: 'github.com/jedipunkz/agx',
                  },
                },
              ],
            },
          },
        ],
      },
    },
    {
      width: 1200,
      height: 630,
      fonts: [
        { name: 'Inter', data: fontRegular, weight: 400, style: 'normal' },
        { name: 'Inter', data: fontBold, weight: 700, style: 'normal' },
      ],
    }
  );

  const resvg = new Resvg(svg);
  const png = resvg.render().asPng();

  return new Response(png, {
    headers: {
      'Content-Type': 'image/png',
      'Cache-Control': 'public, max-age=31536000, immutable',
    },
  });
};
