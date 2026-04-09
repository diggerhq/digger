import { createFileRoute } from '@tanstack/react-router';
import { getOrchestratorBackendUrl, requireUiAuth } from './proxyUtils';

export const Route = createFileRoute('/orchestrator/github/exchange-code')({
  server: {
    handlers: {
      GET: async ({ request }) => {
        try {
          const authResult = await requireUiAuth(request);
          if (authResult instanceof Response) return authResult;

          const url = new URL(request.url);
          const backendUrl = getOrchestratorBackendUrl();

          return await fetch(`${backendUrl}/github/exchange-code${url.search}`, {
            method: 'GET',
            headers: request.headers,
          });
        } catch (error) {
          console.error('Error proxying GitHub exchange-code request:', error);
          return new Response('Internal server error', { status: 500 });
        }
      },
    },
  },
});
