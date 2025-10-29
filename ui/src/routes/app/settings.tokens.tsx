import { createFileRoute, redirect } from '@tanstack/react-router'

export const Route = createFileRoute('/app/settings/tokens')({
  server: {
    handlers: {
      GET: async ({ request }) => {
        return redirect({ to: '/dashboard/settings/tokens' })
      }
    }
  }
})
