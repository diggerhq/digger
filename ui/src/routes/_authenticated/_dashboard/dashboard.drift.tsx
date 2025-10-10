import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/_dashboard/dashboard/drift')({
  component: RouteComponent,
})

function RouteComponent() {
  return <div>Hello "/dashboard/drift"!</div>
}
