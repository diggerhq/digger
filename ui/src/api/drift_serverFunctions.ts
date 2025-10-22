import { createServerFn } from "@tanstack/react-start"
import { testSlackWebhook } from "./drift_slack"

export const testSlackWebhookFn = createServerFn({method: 'POST'})
    .inputValidator((data : {notification_url: string}) => data)
    .handler(async ({ data }) => {
    const response : any = await testSlackWebhook(data.notification_url)
    return response
  })
  