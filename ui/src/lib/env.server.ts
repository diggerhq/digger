// Centralized server-only environment access.
// This module is evaluated once per server process and cached by Node's module system.

import { createServerFn } from "@tanstack/react-start"


// !IMPORTANT: DO NOT ADD ANYTHING SENSITIVE HERE. THIS IS USED ON THE CLIENT SIDE.
export type Env = {
  PUBLIC_URL: string
  PUBLIC_HOSTNAME: string
  STATESMAN_BACKEND_URL: string
  WORKOS_REDIRECT_URI: string
  ORCHESTRATOR_GITHUB_APP_URL: string
  POSTHOG_KEY?: string
  POSTHOG_HOST?: string
}

export const getPublicServerConfig = createServerFn({ method: 'GET' })
  .handler(async ({}) => {
    return {
      PUBLIC_URL: process.env.PUBLIC_URL ?? '',
      PUBLIC_HOSTNAME: process.env.PUBLIC_URL?.replace('https://', '').replace('http://', '') ?? '',
      STATESMAN_BACKEND_URL: process.env.STATESMAN_BACKEND_URL ?? '',
      WORKOS_REDIRECT_URI: process.env.WORKOS_REDIRECT_URI ?? '',
      ORCHESTRATOR_GITHUB_APP_URL: process.env.ORCHESTRATOR_GITHUB_APP_URL ?? '',
      POSTHOG_KEY: process.env.POSTHOG_KEY || process.env.NEXT_PUBLIC_POSTHOG_KEY || process.env.VITE_PUBLIC_POSTHOG_KEY || '',
      POSTHOG_HOST: process.env.POSTHOG_HOST || process.env.NEXT_PUBLIC_POSTHOG_HOST || process.env.VITE_PUBLIC_POSTHOG_HOST || 'https://app.posthog.com',
    } as Env
})