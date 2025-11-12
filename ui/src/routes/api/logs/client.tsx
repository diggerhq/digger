import { createAPIFileRoute } from '@tanstack/react-start/api';

/**
 * API endpoint to receive client-side logs from the browser
 * Forwards them to stdout in Google Cloud Logging format
 */
export const Route = createAPIFileRoute('/api/logs/client')({
  POST: async ({ request }) => {
    try {
      const body = await request.json();
      
      // Validate log entry
      if (!body || typeof body !== 'object') {
        return new Response('Invalid log entry', { status: 400 });
      }

      const {
        level = 'INFO',
        message,
        timestamp,
        url,
        userAgent,
        metadata,
        stack,
      } = body;

      // Map client log level to Google Cloud severity
      const severityMap: Record<string, string> = {
        debug: 'DEBUG',
        info: 'INFO',
        warn: 'WARNING',
        error: 'ERROR',
      };

      // Create structured log entry for Google Cloud Logging
      const logEntry = {
        severity: severityMap[level] || 'INFO',
        message: `[CLIENT] ${message}`,
        timestamp: timestamp || new Date().toISOString(),
        // Add client-specific fields
        httpRequest: {
          userAgent,
          requestUrl: url,
        },
        // Add custom labels for filtering
        labels: {
          source: 'client',
          logType: 'browser',
        },
        // Include metadata if present
        ...(metadata && { metadata }),
        ...(stack && { stack }),
      };

      // Log to stdout (captured by Cloud Logging)
      console.log(JSON.stringify(logEntry));

      // Return success
      return new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    } catch (error: any) {
      // Log error but don't fail the request (silent failure for logging)
      console.error('Failed to process client log:', error);
      return new Response(JSON.stringify({ ok: false }), {
        status: 200, // Still return 200 to avoid client retries
        headers: { 'Content-Type': 'application/json' },
      });
    }
  },
});


