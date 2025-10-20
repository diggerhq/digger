import { createFileRoute, Outlet } from '@tanstack/react-router'

export const Route = createFileRoute(
  '/_authenticated/_dashboard/dashboard/units',
)({
    component: () => <Outlet />,
})
