import { createServerFn } from '@tanstack/react-start';
import { deleteCookie } from '@tanstack/react-start/server';
import { getConfig } from './ssr/config';
import { terminateSession, withAuth } from './ssr/session';
import { getWorkOS } from './ssr/workos';
import type { GetAuthURLOptions, NoUserInfo, UserInfo } from './ssr/interfaces';
import { Organization } from '@workos-inc/node';
import { WidgetScope } from 'node_modules/@workos-inc/node/lib/widgets/interfaces/get-token';

export const getAuthorizationUrl = createServerFn({ method: 'GET' })
  .inputValidator((options?: GetAuthURLOptions) => options)
  .handler(({ data: options = {} }) => {
    const { returnPathname, screenHint, redirectUri } = options;

    return getWorkOS().userManagement.getAuthorizationUrl({
      provider: 'authkit',
      clientId: getConfig('clientId'),
      redirectUri: redirectUri || getConfig('redirectUri'),
      state: returnPathname ? btoa(JSON.stringify({ returnPathname })) : undefined,
      screenHint,
    });
  });

export const getOrganisationDetails = createServerFn({method: 'GET'})
  .inputValidator((data: {organizationId: string}) => data)
  .handler(async ({data: {organizationId}}) : Promise<Organization> => {
  return getWorkOS().organizations.getOrganization(organizationId).then(organization => organization);
});


export const createOrganization = createServerFn({method: 'POST'})
  .inputValidator((data: {name: string, userId: string}) => data)
  .handler(async ({data: {name, userId}}) : Promise<Organization> => {
    try {
      const organization = await getWorkOS().organizations.createOrganization({ name: name });

      await getWorkOS().userManagement.createOrganizationMembership({
        organizationId: organization.id,
        userId: userId,
        roleSlug: "admin",
      });

      return organization;
    } catch (error) {
      console.error('Error creating organization:', error);
      throw error;
    }
  });

export const getSignInUrl = createServerFn({ method: 'GET' })
  .inputValidator((data?: string) => data)
  .handler(async ({ data: returnPathname }) => {
    return await getAuthorizationUrl({ data: { returnPathname, screenHint: 'sign-in' } });
  });

export const getSignUpUrl = createServerFn({ method: 'GET' })
  .inputValidator((data?: string) => data)
  .handler(async ({ data: returnPathname }) => {
    return getAuthorizationUrl({ data: { returnPathname, screenHint: 'sign-up' } });
  });

export const signOut = createServerFn({ method: 'POST' })
  .inputValidator((data?: string) => data)
  .handler(async ({ data: returnTo }) => {
    const cookieName = getConfig('cookieName') || 'wos_session';
    deleteCookie(cookieName);
    await terminateSession({ returnTo });
  });

export const getAuth = createServerFn({ method: 'GET' }).handler(async (): Promise<{auth: UserInfo | NoUserInfo, organisationId: string }> => {
  const auth = await withAuth();
  const organisationId = auth.organizationId!;
  return {auth, organisationId};
});

export const getOrganization = createServerFn({method: 'GET'})
    .inputValidator((data: {organizationId: string}) => data)
    .handler(async ({data: {organizationId}}) : Promise<Organization> => {
  return getWorkOS().organizations.getOrganization(organizationId);
});

export const ensureOrgExists = createServerFn({method: 'GET'})
    .inputValidator((data: {organizationId: string}) => data)
    .handler(async ({data: {organizationId}}) : Promise<Organization> => {
  return getWorkOS().organizations.getOrganization(organizationId);
});


export const getWidgetsAuthToken = createServerFn({method: 'GET'})
    .inputValidator((args: {userId: string, organizationId: string, scopes?: WidgetScope[]}) => args)
    .handler(async ({data: {userId, organizationId, scopes}}) : Promise<string> => {
  return getWorkOS().widgets.getToken({
    userId: userId,
    organizationId: organizationId,
    scopes: scopes ?? ['widgets:users-table:manage'] as WidgetScope[],
  });
})

