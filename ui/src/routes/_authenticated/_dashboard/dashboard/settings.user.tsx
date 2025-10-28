import { WorkosSettings } from '@/components/WorkosSettings'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute(
  '/_authenticated/_dashboard/dashboard/settings/user',
)({
  component: RouteComponent,
  loader: async ({ context }) => {
    const { user, organisationId, role } = context
    return { user, organisationId, role }
  }
})

function RouteComponent() {
  const { user, role, organisationId } = Route.useLoaderData()

  return (
    <WorkosSettings 
      userId={user?.id || ''} 
      email={user?.email || ''} 
      role={role || ''} 
      organisationId={organisationId || ''} 
    />
  )
}