import radixCssUrl from '@radix-ui/themes/styles.css?url';
import workosWidgetsCssUrl from '@workos-inc/widgets/styles.css?url';
import { HeadContent, Outlet, Scripts, createRootRoute } from '@tanstack/react-router';
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools';
import { getAuth, getOrganisationDetails, getSignInUrl } from '@/authkit/serverFunctions';
import type { ReactNode } from 'react';
import { useEffect } from 'react';
import globalCssUrl from '@/styles/global.css?url';
import { Toaster } from '@/components/ui/toaster';
import { getPublicServerConfig } from '@/lib/env.server';
import type { Organization } from '@workos-inc/node';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { initClientLogging } from '@/lib/client-logger';

// PostHog integration
import { PostHogProvider } from 'posthog-js/react';

export const Route = createRootRoute({
  beforeLoad: async () => {
    // Run auth and config in parallel (they don't depend on each other)
    const [authResult, publicServerConfig] = await Promise.all([
      getAuth(),
      getPublicServerConfig()
    ]);
    
    const { auth, organisationId } = authResult;
    
    // Get org details if we have an orgId (this depends on auth)
    let organisationDetails: Organization | null = null;
    if (organisationId) {
      organisationDetails = await getOrganisationDetails({data: {organizationId: organisationId}});
    }
    ;
    return { user: auth.user, organisationId, role: auth.role, organisationName: organisationDetails?.name, publicServerConfig };
  },
  head: () => ({
    meta: [
      {
        charSet: 'utf-8',
      },
      {
        name: 'viewport',
        content: 'width=device-width, initial-scale=1',
      },
      {
        title: 'OpenTaco - Future of IaC is open',
      },
    ],
  }),
  loader: async ({ context }) => {
    const { user, publicServerConfig } = context as any;
    // Only fetch sign-in URL if user is not authenticated
    const url = !user ? await getSignInUrl() : null;
    return {
      user,
      url,
      publicServerConfig,
    } as any;
  },
  component: DashboardRootComponent,
  notFoundComponent: () => <div>Not Found</div>,
});

function DashboardRootComponent() {
  const data = (Route as any).useLoaderData?.() || {};
  return (
    <DashboardRootDocument publicServerConfig={data.publicServerConfig}>
      <Outlet />
      <TanStackRouterDevtools position="bottom-right" />
    </DashboardRootDocument>
  );
}

function DashboardRootDocument({ children, publicServerConfig }: Readonly<{ children: ReactNode, publicServerConfig?: any }>) {
  // Initialize client-side logging once on mount
  useEffect(() => {
    initClientLogging();
  }, []);

  return (
    <html>
      <head>
        <HeadContent />
        {/* Preload and apply critical CSS to avoid FOUC */}
        <link rel="preload" as="style" href={radixCssUrl} />
        <link rel="stylesheet" href={radixCssUrl} />
        <link rel="preload" as="style" href={workosWidgetsCssUrl} />
        <link rel="stylesheet" href={workosWidgetsCssUrl} />
        <link rel="preload" as="style" href={globalCssUrl} />
        <link rel="stylesheet" href={globalCssUrl} />
        {/* App icons */}
        <link rel="icon" type="image/svg+xml" href="/favicon.svg" />
        <link rel="icon" type="image/png" href="/favicon.png" />
        <link rel="apple-touch-icon" href="/favicon.png" />
      </head>
      <body>
        <ErrorBoundary>
          {publicServerConfig?.POSTHOG_KEY ? (
            <PostHogProvider
              apiKey={publicServerConfig.POSTHOG_KEY}
              options={{
                api_host: publicServerConfig.POSTHOG_HOST,
                defaults: '2025-05-24',
                capture_exceptions: true,
                debug: false,
              }}
            >
              {children}
              <Toaster />
              <Scripts />
            </PostHogProvider>
          ) : (
            <>
              {children}
              <Toaster />
              <Scripts />
            </>
          )}
        </ErrorBoundary>
      </body>
    </html>
  );
}