import { createFileRoute } from '@tanstack/react-router';
import { proxyOrchestratorGitHubWebhook } from './proxyUtils';

export const Route = createFileRoute('/orchestrator/github/webhook')({
  server: {
    handlers: {
      POST: async ({ request }) => proxyOrchestratorGitHubWebhook(request),
    },
  },
});
