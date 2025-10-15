import { redirect, createFileRoute } from '@tanstack/react-router';
import { getSignInUrl } from '../authkit/serverFunctions';

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: async ({ context, location }) => {
    if (!context.user) {
      throw redirect({ href: '/' });
    }
  },
});
