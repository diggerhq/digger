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
        
        const pathname = req.url?.split('?')[0] || '/';
        const method = req.method || 'GET';
        
        // Extract user ID and org ID and log request initialization
        const { userId, orgId } = await extractUserInfoFromRequest(req);
        logRequestInit(method, pathname, requestId, userId, orgId);
        
        // Set request ID header
        req.headers['x-request-id'] = requestId;
        
        // Capture original end function to log response
        const originalEnd = res.end.bind(res);
        res.end = function(...args: any[]) {
          const latency = Date.now() - requestStart;
          logResponse(method, pathname, requestId, latency, res.statusCode || 200);
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
      requestLoggingPlugin(),
      tanstackStart(),
      viteReact(),
      // cloudflare({ viteEnvironment: { name: 'ssr' } }),
    ],
  };
});
