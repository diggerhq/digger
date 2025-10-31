import * as React from 'react'
import { useRouter } from '@tanstack/react-router'
import { getWidgetsAuthToken } from '@/authkit/serverFunctions'
import { useToast } from '@/hooks/use-toast'
import { OrganizationSwitcher, WorkOsWidgets } from '@workos-inc/widgets'
import { DropdownMenu } from '@radix-ui/themes'

import '@workos-inc/widgets/styles.css'
import '@radix-ui/themes/styles.css'

type WorkosOrgSwitcherProps = {
  userId: string
  organisationId: string
  label?: string
  redirectTo?: string
  /**
   * If true, wraps the switcher in WorkOsWidgets provider which applies a full-page layout.
   * Leave false for compact embedding in headers/navs to avoid large whitespace.
   */
  wrapWithProvider?: boolean
  /**
   * When true, injects a default extra group with a Settings item in the switcher dropdown.
   */
  showSettingsItem?: boolean
}

export default function WorkosOrgSwitcher({
  userId,
  organisationId,
  label = 'My Orgs',
  redirectTo = '/dashboard/units',
  wrapWithProvider = false,
  showSettingsItem = false,
}: WorkosOrgSwitcherProps) {
  const router = useRouter()
  const { toast } = useToast()
  const [authToken, setAuthToken] = React.useState<string | null>(null)
  const [error, setError] = React.useState<string | null>(null)
  const [loading, setLoading] = React.useState(true)

  const handleSwitchToOrganization = async (organizationId: string) => {
    try {
      const res = await fetch('/api/auth/workos/switch-org', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ organizationId, pathname: redirectTo }),
      })
      const data = await res.json()
      if (!data?.redirectUrl) return
      const url: string = data.redirectUrl
      const isInternal = url.startsWith('/')
      if (isInternal) {
        await router.navigate({ to: url })
        router.invalidate()
      } else {
        throw new Error('Cannot redirect to external URL')
      }
    } catch (e: any) {
      toast({
        title: 'Failed to switch organization',
        description: e?.message ?? 'Failed to switch organization',
        variant: 'destructive',
      })
      console.error('Failed to switch organization', e)
    }
  }

  React.useEffect(() => {
    (async () => {
      try {
        const token = await getWidgetsAuthToken({ data: { userId, organizationId: organisationId } })
        setAuthToken(token)
        setLoading(false)
      } catch (e: any) {
        setError(e?.message ?? 'Failed to get WorkOS token')
        setLoading(false)
      }
    })()
  }, [userId, organisationId])

  if (loading) return <p>Loading WorkOS…</p>
  if (error) return <p className="text-red-600">Error: {error}</p>
  if (!authToken) return <p>Could not load WorkOS token.</p>

  const extraMenu = showSettingsItem ? (
    <>
      <DropdownMenu.Separator />
      <DropdownMenu.Group>
        <DropdownMenu.Item onClick={() => router.navigate({ to: '/dashboard/settings/user' })}>
          Settings
        </DropdownMenu.Item>
      </DropdownMenu.Group>
    </>
  ) : null

  if (wrapWithProvider) {
    return (
      <WorkOsWidgets
        // Reset WorkOS full-page layout styles so it fits inside the sidebar
        style={{ minHeight: 'auto', height: 'auto', padding: 0, display: 'contents' } as any}
      >
        <div className="w-full">
          <OrganizationSwitcher
            authToken={authToken}
            organizationLabel={label}
            switchToOrganization={({ organizationId }) => handleSwitchToOrganization(organizationId)}
          >
            {extraMenu}
          </OrganizationSwitcher>
        </div>
      </WorkOsWidgets>
    )
  }

  return (
    <OrganizationSwitcher
      authToken={authToken}
      organizationLabel={label}
      switchToOrganization={({ organizationId }) => handleSwitchToOrganization(organizationId)}
    >
      {extraMenu}
    </OrganizationSwitcher>
  )
}


