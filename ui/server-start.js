// Simple Node.js HTTP server that runs the TanStack Start fetch handler
import { createServer } from 'node:http';
import { readFile } from 'node:fs/promises';
import { join, extname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { Readable } from 'node:stream';
import { createGzip } from 'node:zlib';
import serverHandler from './dist/server/server.js';

const __dirname = fileURLToPath(new URL('.', import.meta.url));
const PORT = process.env.PORT || 3030;
const HOST = process.env.HOST || '0.0.0.0';
const REQUEST_TIMEOUT = 60 * 1000; // 60s timeout for requests

// Configure global fetch with connection pooling for much better performance
// Without this, every fetch creates a new TCP connection (DNS + handshake overhead)
import { setGlobalDispatcher, Agent } from 'undici';

setGlobalDispatcher(new Agent({
  // Connection pooling - reuse TCP connections
  connections: 100,           // Max connections per origin
  pipelining: 10,             // Max pipelined requests
  
  // Keep connections alive to avoid handshake overhead
  keepAliveTimeout: 60 * 1000,      // Keep idle connections for 60s
  keepAliveMaxTimeout: 10 * 60 * 1000, // Max connection lifetime 10min
  
  // Faster timeouts for better responsiveness
  headersTimeout: 30 * 1000,  // 30s for headers
  bodyTimeout: 30 * 1000,     // 30s for body
  connectTimeout: 10 * 1000,  // 10s to establish connection
}));

console.log('✅ HTTP connection pooling enabled (100 connections, 60s keep-alive)');

// Helper to convert Node.js readable stream to Web ReadableStream
function nodeToWebStream(nodeStream) {
  return new ReadableStream({
    start(controller) {
      nodeStream.on('data', (chunk) => controller.enqueue(chunk));
      nodeStream.on('end', () => controller.close());
      nodeStream.on('error', (err) => controller.error(err));
    },
    cancel() {
      nodeStream.destroy();
    }
  });
}

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
  const requestId = Math.random().toString(36).slice(2, 10);
  
  // Set request timeout
  req.setTimeout(REQUEST_TIMEOUT, () => {
    console.error(`⏱️  Request timeout (${REQUEST_TIMEOUT}ms): ${req.method} ${req.url} [${requestId}]`);
    if (!res.headersSent) {
      res.writeHead(408, { 'Content-Type': 'text/plain' });
      res.end('Request Timeout');
    }
  });
  
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

    // Create Web Standard Request with streaming body (no buffering!)
    // Use duplex: 'half' for proper streaming support
    let body = undefined;
    if (req.method !== 'GET' && req.method !== 'HEAD') {
      body = nodeToWebStream(req);
    }

    const request = new Request(`http://${req.headers.host}${req.url}`, {
      method: req.method,
      headers: req.headers,
      body: body,
      duplex: 'half', // Required for streaming request bodies
    });

    // Call the TanStack Start fetch handler
    const ssrStart = Date.now();
    const response = await serverHandler.fetch(request);
    const ssrTime = Date.now() - ssrStart;
    
    // Log SSR timing with request details for profiling
    if (ssrTime > 2000) {
      console.log(`🔥 VERY SLOW SSR: ${req.method} ${pathname} took ${ssrTime}ms [${requestId}]`);
    } else if (ssrTime > 1000) {
      console.log(`⚠️  SLOW SSR: ${req.method} ${pathname} took ${ssrTime}ms [${requestId}]`);
    } else if (ssrTime > 500) {
      console.log(`⏱️  SSR: ${req.method} ${pathname} took ${ssrTime}ms [${requestId}]`);
    } else if (process.env.DEBUG === 'true') {
      console.log(`✅ SSR: ${req.method} ${pathname} took ${ssrTime}ms [${requestId}]`);
    }

    // Convert Web Standard Response to Node.js response
    res.statusCode = response.status;
    res.statusMessage = response.statusText;

    // Set headers
    response.headers.forEach((value, key) => {
      res.setHeader(key, value);
    });
    
    // Enable HTML caching with must-revalidate for versioned builds
    const contentType = response.headers.get('content-type') || '';
    if (contentType.includes('text/html')) {
      res.setHeader('Cache-Control', 'public, max-age=60, must-revalidate');
    }

    // Check if client accepts gzip and response is compressible
    const acceptEncoding = req.headers['accept-encoding'] || '';
    const shouldCompress = acceptEncoding.includes('gzip') && 
                          (contentType.includes('text/html') || 
                           contentType.includes('application/json') ||
                           contentType.includes('text/css') ||
                           contentType.includes('application/javascript'));

    // Stream the response body with optional compression
    if (response.body) {
      if (shouldCompress) {
        // Compress the response
        res.setHeader('Content-Encoding', 'gzip');
        res.removeHeader('Content-Length'); // Let gzip set the correct length
        
        const gzip = createGzip();
        gzip.pipe(res);
        
        const reader = response.body.getReader();
        try {
          while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            gzip.write(value);
          }
          gzip.end();
        } catch (err) {
          console.error(`Stream error [${requestId}]:`, err);
          gzip.destroy();
          if (!res.headersSent) {
            res.statusCode = 500;
            res.end();
          }
        }
      } else {
        // Stream without compression
        const reader = response.body.getReader();
        try {
          while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            res.write(value);
          }
          res.end();
        } catch (err) {
          console.error(`Stream error [${requestId}]:`, err);
          if (!res.headersSent) {
            res.statusCode = 500;
            res.end();
          }
        }
      }
    } else {
      res.end();
    }
  } catch (error) {
    console.error(`Server error [${requestId}]:`, error);
    if (!res.headersSent) {
      res.statusCode = 500;
      res.setHeader('Content-Type', 'text/plain');
      res.end('Internal Server Error');
    }
  }
});

// Configure server keep-alive for client connections
server.keepAliveTimeout = 65 * 1000; // 65s (longer than typical load balancer timeout)
server.headersTimeout = 66 * 1000;   // Must be > keepAliveTimeout

server.listen(PORT, HOST, () => {
  console.log(`🚀 Server running at http://${HOST}:${PORT}/`);
  console.log(`   ✓ Keep-alive: ${server.keepAliveTimeout}ms`);
  console.log(`   ✓ Request timeout: ${REQUEST_TIMEOUT}ms`);
  console.log(`   ✓ Gzip compression: enabled`);
  console.log(`   ✓ Streaming request bodies: enabled`);
});

