// Simple Node.js HTTP server that runs the TanStack Start fetch handler
import { createServer } from 'node:http';
import { readFile } from 'node:fs/promises';
import { join, extname } from 'node:path';
import { fileURLToPath } from 'node:url';
import serverHandler from './dist/server/server.js';

const __dirname = fileURLToPath(new URL('.', import.meta.url));
const PORT = process.env.PORT || 3030;
const HOST = process.env.HOST || '0.0.0.0';

// MIME type mapping
const MIME_TYPES = {
  '.html': 'text/html',
  '.js': 'application/javascript',
  '.mjs': 'application/javascript',
  '.css': 'text/css',
  '.json': 'application/json',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.gif': 'image/gif',
  '.svg': 'image/svg+xml',
  '.ico': 'image/x-icon',
  '.woff': 'font/woff',
  '.woff2': 'font/woff2',
};

const server = createServer(async (req, res) => {
  try {
    const url = new URL(req.url, `http://${req.headers.host}`);
    const pathname = url.pathname;

    // Try to serve static files from dist/client first
    // Serve: /assets/*, *.js, *.css, *.json, images, fonts, favicons
    const staticExtensions = ['.js', '.mjs', '.css', '.json', '.png', '.jpg', '.gif', '.ico', '.svg', '.woff', '.woff2'];
    const isStaticFile = pathname.startsWith('/assets/') || 
                         staticExtensions.some(ext => pathname.endsWith(ext));
    
    if (isStaticFile) {
      try {
        const filePath = join(__dirname, 'dist', 'client', pathname);
        const content = await readFile(filePath);
        const ext = extname(pathname);
        const mimeType = MIME_TYPES[ext] || 'application/octet-stream';
        
        res.writeHead(200, {
          'Content-Type': mimeType,
          'Cache-Control': 'public, max-age=31536000, immutable',
        });
        res.end(content);
        return;
      } catch (err) {
        // File not found, fall through to SSR handler
      }
    }

    // Get request body if present
    let body = undefined;
    if (req.method !== 'GET' && req.method !== 'HEAD') {
      const chunks = [];
      for await (const chunk of req) {
        chunks.push(chunk);
      }
      body = Buffer.concat(chunks);
    }

    // Create Web Standard Request
    const request = new Request(`http://${req.headers.host}${req.url}`, {
      method: req.method,
      headers: req.headers,
      body: body,
    });

    // Call the TanStack Start fetch handler
    const ssrStart = Date.now();
    const response = await serverHandler.fetch(request);
    const ssrTime = Date.now() - ssrStart;
    
    // Log slow SSR requests
    if (ssrTime > 1000) {
      console.log(`⚠️  SLOW SSR: ${req.method} ${pathname} took ${ssrTime}ms`);
    }

    // Convert Web Standard Response to Node.js response
    res.statusCode = response.status;
    res.statusMessage = response.statusText;

    // Set headers
    response.headers.forEach((value, key) => {
      res.setHeader(key, value);
    });

    // Stream the response body
    if (response.body) {
      const reader = response.body.getReader();
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        res.write(value);
      }
    }
    
    res.end();
  } catch (error) {
    console.error('Server error:', error);
    res.statusCode = 500;
    res.end('Internal Server Error');
  }
});

server.listen(PORT, HOST, () => {
  console.log(`🚀 Server running at http://${HOST}:${PORT}/`);
});

