import { createFileRoute } from '@tanstack/react-router';
import {
  getOrchestratorBackendUrl,
  requireUiAuth,
  withForwardedHostProto,
} from './proxyUtils';

export const Route = createFileRoute('/orchestrator/github/setup')({
  server: {
    handlers: {
      GET: async ({ request }) => {
        try {
          const authResult = await requireUiAuth(request);
          if (authResult instanceof Response) return authResult;

          const url = new URL(request.url);
          const headers = withForwardedHostProto(request);
          const backendUrl = getOrchestratorBackendUrl();

          return await fetch(`${backendUrl}/github/setup${url.search}`, {
            method: 'GET',
            headers,
          });
        } catch (error) {
          console.error('Error proxying GitHub setup request:', error);
          return new Response('Internal server error', { status: 500 });
        }
      },
    },
  },
});
