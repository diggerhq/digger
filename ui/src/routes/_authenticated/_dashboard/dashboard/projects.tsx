import { createFileRoute, Outlet } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/_dashboard/dashboard/projects',)({
  component: () => <Outlet />,
})
