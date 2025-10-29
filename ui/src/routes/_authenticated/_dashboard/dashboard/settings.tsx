import { createFileRoute, Link, Outlet, redirect, useLocation } from '@tanstack/react-router'
import { cn } from '@/lib/utils'

export const Route = createFileRoute(
  '/_authenticated/_dashboard/dashboard/settings',
)({
  component: RouteComponent,
  loader: async ({ context }) => {
    const { user, organisationId, role } = context
    return { user, organisationId, role }
  },
  beforeLoad: ({ location, search }) => {
    if (location.pathname === '/dashboard/settings') {
      throw redirect({
        to: '/dashboard/settings/user',
        search
      })
    }
    return {}
  }
})

function RouteComponent() {
  const data = Route.useLoaderData()
  const location = useLocation()
  const isTokensPage = location.pathname.includes('tokens')

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-semibold tracking-tight">Settings</h2>
        <p className="text-sm text-muted-foreground">
          Manage your account settings and API tokens
        </p>
      </div>
      
      <div className="flex justify-start border-b">
        <nav className="flex space-x-2">
          <Link
            to="/dashboard/settings/user"
            className={cn(
              "flex items-center gap-2 px-4 py-2 text-sm font-medium border-b-2 -mb-px",
              !isTokensPage 
                ? "border-primary text-primary" 
                : "border-transparent text-muted-foreground hover:text-foreground hover:border-muted"
            )}
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              className="w-4 h-4"
            >
              <path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2" />
              <circle cx="12" cy="7" r="4" />
            </svg>
            User Settings
          </Link>
          <Link
            to="/dashboard/settings/tokens"
            className={cn(
              "flex items-center gap-2 px-4 py-2 text-sm font-medium border-b-2 -mb-px",
              isTokensPage 
                ? "border-primary text-primary" 
                : "border-transparent text-muted-foreground hover:text-foreground hover:border-muted"
            )}
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              className="w-4 h-4"
            >
              <path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4" />
            </svg>
            API Tokens
          </Link>
        </nav>
      </div>

      <div className="flex-1">
        <Outlet />
      </div>
    </div>
  )
}