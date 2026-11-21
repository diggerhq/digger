import { defineConfig, loadEnv } from 'vite';
import tsConfigPaths from 'vite-tsconfig-paths';
import { tanstackStart } from '@tanstack/react-start/plugin/vite';
import viteReact from '@vitejs/plugin-react';
import { extractUserInfoFromRequest, logRequestInit, logResponse } from './request-logging.js';
import type { Plugin } from 'vite';

// Logging middleware plugin
function requestLoggingPlugin(): Plugin {
  return {
    name: 'request-logging',
    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        const existingRequestId = req.headers['x-request-id'];
        const requestId = Array.isArray(existingRequestId) 
          ? existingRequestId[0] 
          : existingRequestId || `ssr-${Math.random().toString(36).slice(2, 10)}`;
        const requestStart = Date.now();
        
        const fullUrl = req.url || '/';
        const pathname = fullUrl.split('?')[0];
        const method = req.method || 'GET';
        
        // Skip logging for TanStack Router component split requests and Vite internal requests
        // Check full URL for query params, pathname for path patterns
        const isTanStackSplitRequest = fullUrl.includes('tsr-split=component') || pathname.startsWith('/src/routes/');
        const isViteInternal = pathname.startsWith('/@') || pathname.startsWith('/node_modules/') || pathname.startsWith('/@fs/');
        
        if (!isTanStackSplitRequest && !isViteInternal) {
          // Extract user ID and org ID and log request initialization
          try {
            const { userId, orgId } = await extractUserInfoFromRequest(req);
            logRequestInit(method, pathname, requestId, userId, orgId);
          } catch (error) {
            // Silently fail logging for dev server requests
          }
        }
        
        // Set request ID header
        req.headers['x-request-id'] = requestId;
        
        // Capture original end function to log response
        const originalEnd = res.end.bind(res);
        res.end = function(...args: any[]) {
          if (!isTanStackSplitRequest && !isViteInternal) {
            try {
              const latency = Date.now() - requestStart;
              logResponse(method, pathname, requestId, latency, res.statusCode || 200);
            } catch (error) {
              // Silently fail logging for dev server requests
            }
          }
          return originalEnd(...args);
        };
        
        next();
      });
    },
  };
}

export default defineConfig(({ mode }) => {
  
  const env = loadEnv(mode, process.cwd(), '');
  const allowedHosts = (env.ALLOWED_HOSTS || '')
    .split(',')
    .map((h) => h.trim())
    .filter(Boolean);

  return {
    base: '/',
    build: {
      // Ensure proper asset handling for TanStack Start
      rollupOptions: {
        output: {
          // Ensure consistent asset naming
          assetFileNames: 'assets/[name]-[hash][extname]',
          chunkFileNames: 'assets/[name]-[hash].js',
          entryFileNames: 'assets/[name]-[hash].js',
        },
      },
    },
    ssr: {
      // Force native Node resolution at runtime (no inlining)
      external: ['@workos-inc/node'],
      // Do NOT list it in noExternal (that would inline/transform it)
    },   
    server: {
      port: 3030,
      allowedHosts,
      // Ensure proper handling of dynamic imports and component splitting
      hmr: {
        protocol: 'ws',
      },
    },
    plugins: [
      tsConfigPaths({
        projects: ['./tsconfig.json'],
      }),
      // TanStack Start must come before request logging to handle component splitting
      tanstackStart(),
      requestLoggingPlugin(),
      viteReact(),
      // cloudflare({ viteEnvironment: { name: 'ssr' } }),
    ],
  };
});
