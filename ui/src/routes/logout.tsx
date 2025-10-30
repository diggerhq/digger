import { createFileRoute } from '@tanstack/react-router';
import { signOut } from '../authkit/serverFunctions';

export const Route = createFileRoute('/logout')({
  preload: false,
  loader: async ({context}) => {
    const redirectUrl = context.publicServerConfig.PUBLIC_URL
    await signOut({ data: redirectUrl });
  },
});
