import { createFileRoute } from '@tanstack/react-router';


export const Route = createFileRoute('/orchestrator/github/webhook')({
  server: {
    handlers: {
      POST: async ({ request }) => {
        try {
          const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/github-app-webhook`, {
            method: 'POST',
            headers: request.headers,
            body: request.body,
            // @ts-expect-error: 'duplex' is required by Node/undici for streaming bodies
            duplex: 'half',
          });

          return response;
        } catch (error) {
          console.error('Error in POST handler:', error);
          return new Response('Internal server error', { status: 500 });
        }
      },
    },
  },
});