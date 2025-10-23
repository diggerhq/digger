import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/_well-known/terraform_json')({
  server: {
    handlers: {
      GET: async ({ request }) => {
        try {
          const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}/.well-known/terraform.json`, {
            method: 'GET',
            headers: request.headers,
          });
          return response;
        } catch (error) {
          console.error('Error in GET handler:', error);
          return new Response('Internal server error', { status: 500 });
        }
      },
    },
  },
});
