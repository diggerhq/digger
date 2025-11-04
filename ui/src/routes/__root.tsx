import radixCssUrl from '@radix-ui/themes/styles.css?url';
import workosWidgetsCssUrl from '@workos-inc/widgets/styles.css?url';
import { HeadContent, Outlet, Scripts, createRootRoute } from '@tanstack/react-router';
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools';
import { Suspense } from 'react';
import { getAuth, getOrganisationDetails, getSignInUrl } from '@/authkit/serverFunctions';
import type { ReactNode } from 'react';
import globalCssUrl from '@/styles/global.css?url'
import { Toaster } from '@/components/ui/toaster';
import { getPublicServerConfig } from '@/lib/env.server';
import type { Organization } from '@workos-inc/node';



export const Route = createRootRoute({
  beforeLoad: async () => {
    const startRootLoad = Date.now();
    
    // Run auth and config in parallel (they don't depend on each other)
    const parallelStart = Date.now();
    const [authResult, publicServerConfig] = await Promise.all([
      getAuth(),
      getPublicServerConfig()
    ]);
    const parallelTime = Date.now() - parallelStart;
    
    const { auth, organisationId } = authResult;
    
    // Get org details if we have an orgId (this depends on auth)
    let organisationDetails: Organization | null = null;
    let orgTime = 0;
    if (organisationId) {
      const orgStart = Date.now();
      organisationDetails = await getOrganisationDetails({data: {organizationId: organisationId}});
      orgTime = Date.now() - orgStart;
    }
    
    const totalTime = Date.now() - startRootLoad;
    if (totalTime > 500) {
      console.log(`⚠️  Root loader took ${totalTime}ms (parallel: ${parallelTime}ms, org: ${orgTime}ms)`);
    }
    
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
    const { user } = context;
    // Only fetch sign-in URL if user is not authenticated
    const url = !user ? await getSignInUrl() : null;
    return {
      user,
      url,
    };
  },
  component: DashboardRootComponent,
  notFoundComponent: () => <div>Not Found</div>,
});

function DashboardRootComponent() {
  return (
    <DashboardRootDocument>
      <Outlet />
      <Suspense fallback={null}>
        <TanStackRouterDevtools position="bottom-right" />
      </Suspense>
    </DashboardRootDocument>
  );
}

function DashboardRootDocument({ children }: Readonly<{ children: ReactNode }>) {
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
        {children}
        <Toaster />
        <Scripts />
      </body>
    </html>
  );
}
