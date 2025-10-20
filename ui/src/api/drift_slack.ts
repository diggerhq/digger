
export async function testSlackWebhook(
    notification_url: string
  ) {
    const response = await fetch(`${process.env.DRIFT_REPORTING_BACKEND_URL}/_internal/send_test_slack_notification_for_url`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${process.env.DRIFT_REPORTING_BACKEND_WEBHOOK_SECRET}`,
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