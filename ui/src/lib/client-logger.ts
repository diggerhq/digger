/**
 * Client-side logging utility for capturing browser logs and sending to backend
 * This captures console.log, console.error, React errors, and unhandled exceptions
 */

import posthog from 'posthog-js';

type LogLevel = 'debug' | 'info' | 'warn' | 'error';

interface LogEntry {
  level: LogLevel;
  message: string;
  timestamp: string;
  url?: string;
  userAgent?: string;
  metadata?: Record<string, any>;
  stack?: string;
}

// Check if we're in the browser
const isBrowser = typeof window !== 'undefined';

// Store original console methods
const originalConsole = isBrowser ? {
  log: console.log,
  error: console.error,
  warn: console.warn,
  debug: console.debug,
} : null;

/**
 * Send log to backend for Google Cloud Logging
 */
async function sendLogToBackend(entry: LogEntry) {
  // Only send error and warn logs to backend to avoid spam
  if (entry.level !== 'error' && entry.level !== 'warn') return;
  
  try {
    // Send to backend endpoint (you'll need to create this)
    await fetch('/api/logs/client', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(entry),
      // Don't wait for response, fire and forget
      keepalive: true,
    }).catch(() => {
      // Silently fail - don't want logging to break the app
    });
  } catch {
    // Ignore errors from logging
  }
}

/**
 * Capture log entry and send to PostHog + backend
 */
function captureLog(level: LogLevel, args: any[]) {
  if (!isBrowser) return;

  // Format message from arguments
  const message = args
    .map(arg => {
      if (typeof arg === 'object') {
        try {
          return JSON.stringify(arg, null, 2);
        } catch {
          return String(arg);
        }
      }
      return String(arg);
    })
    .join(' ');

  const entry: LogEntry = {
    level,
    message,
    timestamp: new Date().toISOString(),
    url: window.location.href,
    userAgent: navigator.userAgent,
  };

  // Extract error stack if present
  const errorArg = args.find(arg => arg instanceof Error);
  if (errorArg) {
    entry.stack = errorArg.stack;
    entry.metadata = {
      errorName: errorArg.name,
      errorMessage: errorArg.message,
    };
  }

  // Send to PostHog if available and it's an error/warn
  if (level === 'error' || level === 'warn') {
    try {
      if ((posthog as any)?.__loaded) {
        posthog.capture(`client_${level}`, {
          message: entry.message,
          stack: entry.stack,
          url: entry.url,
        });
      }
    } catch {
      // Ignore PostHog errors
    }
  }

  // Send to backend for Cloud Logging
  sendLogToBackend(entry);
}

/**
 * Override console methods to capture logs
 */
export function initClientLogging() {
  if (!isBrowser || !originalConsole) return;

  // Override console.log
  console.log = (...args: any[]) => {
    originalConsole!.log(...args);
    captureLog('info', args);
  };

  // Override console.error
  console.error = (...args: any[]) => {
    originalConsole!.error(...args);
    captureLog('error', args);
  };

  // Override console.warn
  console.warn = (...args: any[]) => {
    originalConsole!.warn(...args);
    captureLog('warn', args);
  };

  // Override console.debug
  console.debug = (...args: any[]) => {
    originalConsole!.debug(...args);
    captureLog('debug', args);
  };

  // Capture unhandled errors
  window.addEventListener('error', (event) => {
    const entry: LogEntry = {
      level: 'error',
      message: event.message,
      timestamp: new Date().toISOString(),
      url: window.location.href,
      userAgent: navigator.userAgent,
      stack: event.error?.stack,
      metadata: {
        filename: event.filename,
        lineno: event.lineno,
        colno: event.colno,
      },
    };

    // Send to PostHog
    try {
      if ((posthog as any)?.__loaded) {
        posthog.capture('client_unhandled_error', entry);
      }
    } catch {}

    // Send to backend
    sendLogToBackend(entry);
  });

  // Capture unhandled promise rejections
  window.addEventListener('unhandledrejection', (event) => {
    const entry: LogEntry = {
      level: 'error',
      message: `Unhandled Promise Rejection: ${event.reason}`,
      timestamp: new Date().toISOString(),
      url: window.location.href,
      userAgent: navigator.userAgent,
      stack: event.reason?.stack,
    };

    // Send to PostHog
    try {
      if ((posthog as any)?.__loaded) {
        posthog.capture('client_unhandled_rejection', entry);
      }
    } catch {}

    // Send to backend
    sendLogToBackend(entry);
  });

  originalConsole.log('✅ Client-side logging initialized');
}

/**
 * Restore original console methods (useful for cleanup)
 */
export function restoreConsole() {
  if (!isBrowser || !originalConsole) return;
  
  console.log = originalConsole.log;
  console.error = originalConsole.error;
  console.warn = originalConsole.warn;
  console.debug = originalConsole.debug;
}

/**
 * Manual log function for structured logging
 */
export function logClient(level: LogLevel, message: string, metadata?: Record<string, any>) {
  const entry: LogEntry = {
    level,
    message,
    timestamp: new Date().toISOString(),
    url: isBrowser ? window.location.href : undefined,
    userAgent: isBrowser ? navigator.userAgent : undefined,
    metadata,
  };

  // Send to PostHog if error/warn
  if (level === 'error' || level === 'warn') {
    try {
      if (isBrowser && (posthog as any)?.__loaded) {
        posthog.capture(`client_${level}`, entry);
      }
    } catch {}
  }

  // Send to backend
  if (isBrowser) {
    sendLogToBackend(entry);
  }

  // Also log to console
  if (originalConsole) {
    const method = originalConsole[level === 'debug' ? 'debug' : level === 'warn' ? 'warn' : level === 'error' ? 'error' : 'log'];
    method(`[${level.toUpperCase()}]`, message, metadata || '');
  }
}


