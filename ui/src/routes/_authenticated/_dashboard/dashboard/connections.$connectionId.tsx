import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute(
  '/_authenticated/_dashboard/dashboard/connections/$connectionId',
)({
  component: RouteComponent,
})

function RouteComponent() {
  return (
    <div>
      Hello "/_authenticated/_dashboard/dashboard/connections/$connectionId"!
    </div>
  )
}
