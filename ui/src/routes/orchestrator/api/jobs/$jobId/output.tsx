import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/orchestrator/api/jobs/$jobId/output')({
  server: {
    handlers: {
      GET: async ({ request, params }) => {
        try {
          const url = new URL(request.url)
          const queryString = url.search
          const response = await fetch(
            `${process.env.ORCHESTRATOR_BACKEND_URL}/orchestrator/api/jobs/${params.jobId}/output${queryString}`,
            {
              method: 'GET',
              headers: {
                'X-Request-ID':
                  request.headers.get('x-request-id') ||
                  `proxy-${Date.now()}`,
              },
            },
          )
          return response
        } catch (error) {
          console.error('Error proxying job output:', error)
          return new Response('Bad Gateway', { status: 502 })
        }
      },
    },
  },
})
