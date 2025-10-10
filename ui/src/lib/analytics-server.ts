 import { PostHog } from 'posthog-node'

export const posthog = new PostHog(
  process.env.NEXT_PUBLIC_POSTHOG_KEY!, // Your PostHog project API key
  {
    host: process.env.NEXT_PUBLIC_POSTHOG_HOST || 'https://app.posthog.com',
  }
)


export async function trackEvent(eventName: string, user: any, organizationId: string, properties: Record<string, any> = {}) {
  try {
    posthog.capture({
      distinctId: user.id,
      event: eventName,
      properties: {
        email: user.email,
        user_id: user.id,
        organization_id: organizationId,
        ...properties
      }
    })

    await posthog
      .flush()
      .then(() => console.log('Flushed OK'))
      .catch((err) => console.error('Flush error:', err));

  } catch (error) {
    console.error('PostHog error:', error);
    // Don't fail the entire request due to analytics
  }
}


export async function sendGithubInstallationEvent(installationId: string, user: any, organizationId: string) {
    await trackEvent('github_app_installation_complete', user, organizationId, {
      installation_id: installationId,
      status: 'success'
    });
}