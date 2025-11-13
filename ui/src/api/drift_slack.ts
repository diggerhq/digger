// Helper to generate request IDs for tracing
function generateRequestId(): string {
    return `ui-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

export async function testSlackWebhook(
    notification_url: string
  ) {
    const response = await fetch(`${process.env.DRIFT_REPORTING_BACKEND_URL}/_internal/send_test_slack_notification_for_url`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${process.env.DRIFT_REPORTING_BACKEND_WEBHOOK_SECRET}`,
        'X-Request-ID': generateRequestId(),
      },
      body: JSON.stringify({
        notification_url: notification_url
      })
    })
  
    console.log(response)
    if (!response.ok) {
      const text = await response.text()
      console.log(text)
      throw new Error('Failed to test Slack webhook')
    }
  
    return response.text()
  } 