import { createFileRoute } from '@tanstack/react-router';
import { proxyOrchestratorGitHubWebhook } from './github/proxyUtils';

// Deprecated legacy webhook path kept for backward compatibility.
// We standardized GitHub App endpoints under /orchestrator/github/* and now
// use /orchestrator/github/webhook as canonical. Existing GitHub Apps may
// still be configured with /orchestrator/github-app-webhook, so keep this
// route active and forward it to the same backend handler.
export const Route = createFileRoute('/orchestrator/github-app-webhook')({
  server: {
    handlers: {
      POST: async ({ request }) => proxyOrchestratorGitHubWebhook(request),
    },
  },
});
