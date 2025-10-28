import * as React from 'react';
import { createFileRoute, useRouter } from '@tanstack/react-router';
import { getWidgetsAuthToken } from '@/authkit/serverFunctions';

import {
  OrganizationSwitcher,
  UserProfile,
  UserSecurity,
  WorkOsWidgets,
  UsersManagement,
} from '@workos-inc/widgets';

import '@workos-inc/widgets/styles.css';
import '@radix-ui/themes/styles.css';
import CreateOrganizationBtn from './CreateOrganisationButtonWOS';


type LoaderData = {
  organisationId: string;
  role: 'admin' | 'member' | string;
  // You can also supply userId from your auth loader if you want
  userId: string;
};




type WorkosSettingsProps = {
  userId: string;
  email: string;
  organisationId: string;
  role: 'admin' | 'member' | string;
};

export function WorkosSettings({ userId, email, organisationId, role }: WorkosSettingsProps) {
  const router = useRouter()
  const [authToken, setAuthToken] = React.useState<string | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [loading, setLoading] = React.useState(true);
  const handleSwitchToOrganization = async (organizationId: string) => {

      try {
        const res = await fetch('/api/auth/workos/switch-org', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ organizationId, pathname: '/dashboard/units' }),
        })
        const data = await res.json()
        if (!data?.redirectUrl) return
        const url: string = data.redirectUrl
        const isInternal = url.startsWith('/')
        if (isInternal) {
          await router.navigate({ to: url })
          router.invalidate()
        } else {
          window.location.href = url
        }
      } catch (e) {
        console.error('Failed to switch organization', e)
      }

  }

  React.useEffect(() => {
    (async () => {
      try {
        const authToken = await getWidgetsAuthToken({ data: { userId, organizationId: organisationId } });
        setAuthToken(authToken);
        setLoading(false);
      } catch (e: any) {
        setError(e?.message ?? 'Failed to get WorkOS token');
        setLoading(false);
      }
    })();
  }, [userId, organisationId]);


  if (loading) return <p>Loading WorkOS…</p>;
  if (error) return <p className="text-red-600">Error: {error}</p>;
  if (!authToken) return <p>Could not load WorkOS token.</p>;

  return (
    <div className="max-w-3xl mx-auto py-6">
      <WorkOsWidgets>
        <OrganizationSwitcher
          authToken={authToken}
          organizationLabel="My Orgs"
          switchToOrganization={({ organizationId }) => handleSwitchToOrganization(organizationId)}
        />
        <div className="h-4" />
        {/* Add your org creation UI here */}
        <CreateOrganizationBtn userId={userId} email={email} />
        <div className="h-4" />
        <UserProfile authToken={authToken} />
        <div className="h-4" />
        <UserSecurity authToken={authToken} />
        <div className="h-4" />
        {role === 'admin' && <UsersManagement authToken={authToken} />}
      </WorkOsWidgets>
    </div>
  );
}