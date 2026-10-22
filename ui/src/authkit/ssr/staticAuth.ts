import type { Organization, User } from '@workos-inc/node';
import type { UserInfo } from './interfaces';

/**
 * Optional single-tenant auth mode for self-hosted deployments that don't use WorkOS.
 *
 * Enabled with DIGGER_UI_AUTH_MODE=static. The UI then runs as one fixed identity taken
 * from DIGGER_STATIC_* env vars and never calls WorkOS, so no WORKOS_* env is required to
 * boot. Intended for deployments where access is already controlled in front of the UI
 * (network perimeter, VPN, or an authenticating reverse proxy).
 *
 * DIGGER_STATIC_ORG_ID is required: it must be the backend organisation's external id so
 * org lookups resolve. Deployments whose org was created outside WorkOS (e.g. the GitHub
 * OAuth path) should also set DIGGER_ORG_SOURCE to match the org's external source.
 */
export function isStaticAuthMode(): boolean {
  return process.env.DIGGER_UI_AUTH_MODE === 'static';
}

export function getStaticUserInfo(): UserInfo {
  const organizationId = process.env.DIGGER_STATIC_ORG_ID;
  if (!organizationId) {
    throw new Error('DIGGER_UI_AUTH_MODE=static requires DIGGER_STATIC_ORG_ID (the backend organisation external id)');
  }
  return {
    sessionId: 'static-session',
    user: {
      id: process.env.DIGGER_STATIC_USER_ID || 'static-user',
      email: process.env.DIGGER_STATIC_USER_EMAIL || 'admin@example.com',
      firstName: process.env.DIGGER_STATIC_USER_FIRST_NAME || 'Self-Hosted',
      lastName: process.env.DIGGER_STATIC_USER_LAST_NAME || 'Admin',
    } as unknown as User,
    organizationId,
    role: 'admin',
    permissions: [],
    entitlements: [],
    impersonator: undefined,
    accessToken: '',
  };
}

export function getStaticOrganisation(organizationId: string): Organization {
  return {
    id: organizationId,
    name: process.env.DIGGER_STATIC_ORG_NAME || 'Self-Hosted',
  } as unknown as Organization;
}

/**
 * Value for the DIGGER_ORG_SOURCE header sent to the backend. Defaults to 'workos';
 * self-hosted deployments whose org has a different external source (e.g. 'digger' for
 * orgs created via the GitHub OAuth path) can override it with the DIGGER_ORG_SOURCE env.
 */
export function getOrgSource(): string {
  return process.env.DIGGER_ORG_SOURCE || 'workos';
}
