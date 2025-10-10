import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

export default function GitLabConnection() {
  return (
    <div className="space-y-4">
      <p>To connect your GitLab repository, you need to add a webhook and provide your access token.</p>
      <div>
        <Label htmlFor="webhook-url">Webhook URL</Label>
        <Input id="webhook-url" value="https://your-app.com/gitlab-webhook" readOnly />
      </div>
      <div>
        <Label htmlFor="access-token">GitLab Access Token</Label>
        <Input id="access-token" type="password" placeholder="Enter your GitLab access token" />
      </div>
    </div>
  )
}

