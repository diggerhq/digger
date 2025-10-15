import * as React from 'react';
import { createFileRoute } from '@tanstack/react-router';
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
  organisationId: string;
  role: 'admin' | 'member' | string;
};

export function WorkosSettings({ userId, organisationId, role }: WorkosSettingsProps) {
  const [authToken, setAuthToken] = React.useState<string | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [loading, setLoading] = React.useState(true);

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
          switchToOrganization={async ({ organizationId }) => {
            // Call your own server action if needed
          }}
        />
        <div className="h-4" />
        {/* Add your org creation UI here */}
        <CreateOrganizationBtn userId={userId} />
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