// Centralized server-only environment access.
// This module is evaluated once per server process and cached by Node's module system.

import { createServerFn } from "@tanstack/react-start"


export type Env = {
  PUBLIC_URL: string
  PUBLIC_HOSTNAME: string
  STATESMAN_BACKEND_URL: string
}

export const getPublicServerConfig = createServerFn({ method: 'GET' })
  .handler(async ({}) => {
    return {
      PUBLIC_URL: process.env.PUBLIC_URL ?? '',
      PUBLIC_HOSTNAME: process.env.PUBLIC_URL?.replace('https://', '').replace('http://', '') ?? '',
      STATESMAN_BACKEND_URL: process.env.STATESMAN_BACKEND_URL ?? '',
    } as Env
})