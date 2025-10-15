import { defineConfig, loadEnv } from 'vite';
import tsConfigPaths from 'vite-tsconfig-paths';
import { tanstackStart } from '@tanstack/react-start/plugin/vite';
import viteReact from '@vitejs/plugin-react';
import netlify from '@netlify/vite-plugin-tanstack-start';

export default defineConfig(({ mode }) => {
  
  const env = loadEnv(mode, process.cwd(), '');
  const allowedHosts = (env.ALLOWED_HOSTS || '')
    .split(',')
    .map((h) => h.trim())
    .filter(Boolean);

  return {
    ssr: {
      // Force native Node resolution at runtime (no inlining)
      external: ['@workos-inc/node'],
      // Do NOT list it in noExternal (that would inline/transform it)
    },   
    server: {
      port: 3030,
      allowedHosts,
    },
    plugins: [
      tsConfigPaths({
        projects: ['./tsconfig.json'],
      }),
      netlify(),
      // cloudflare({ viteEnvironment: { name: 'ssr' } }),
      tanstackStart(),
      viteReact(),
    ],
  };
});
