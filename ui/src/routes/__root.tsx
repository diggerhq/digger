import { Box, Button, Card, Container, Flex, Theme } from '@radix-ui/themes';
import radixCssUrl from '@radix-ui/themes/styles.css?url';
import { HeadContent, Link, Outlet, Scripts, createRootRoute } from '@tanstack/react-router';
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools';
import { Suspense } from 'react';
import { getAuth, getSignInUrl, signOut } from '@/authkit/serverFunctions';
import Footer from '@/components/footer';
import SignInButton from '@/components/sign-in-button';
import type { ReactNode } from 'react';
import { Sidebar, SidebarMenuButton, SidebarGroupContent, SidebarGroupLabel, SidebarGroup, SidebarContent, SidebarHeader, SidebarTrigger, SidebarProvider, SidebarMenu, SidebarMenuItem } from '../components/ui/sidebar';
import { GitBranch, Folders, Waves, Settings, CreditCard, LogOut } from 'lucide-react';
import globalCssUrl from '@/styles/global.css?url'


export const Route = createRootRoute({
  beforeLoad: async () => {
    const { auth, organisationId } = await getAuth();
    return { user: auth.user, organisationId };
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
    const url = await getSignInUrl();
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
      <TanStackRouterDevtools position="bottom-right" />
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
        <link rel="preload" as="style" href={globalCssUrl} />
        <link rel="stylesheet" href={globalCssUrl} />
        {/* App icons */}
        <link rel="icon" type="image/svg+xml" href="/favicon.svg" />
        <link rel="icon" type="image/png" href="/favicon.png" />
        <link rel="apple-touch-icon" href="/favicon.png" />
      </head>
      <body>
        {children}
        <Scripts />
      </body>
    </html>
  );
}
