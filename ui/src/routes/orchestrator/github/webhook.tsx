import { createFileRoute } from '@tanstack/react-router';


export const Route = createFileRoute('/orchestrator/github/webhook')({
  server: {
    handlers: {
      POST: async ({ request }) => {
        try {
          
          const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/github/webhook`, {
            method: 'POST',
            headers: request.headers,
            body: request.body,
          }).then(response => response.json())
          return response
        } catch (error) {
          console.error('Error in POST handler:', error);
          return { error: 'Internal server error' }
        }
      },
    },
  },
});