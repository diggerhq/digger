import { createFileRoute, Link } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { CheckCircle, AlertCircle, Save, TestTube, Clock, ArrowLeft, Slack } from 'lucide-react'
import { OrgSettings } from '@/api/types'
import { useState } from 'react'
import { useToast } from '@/hooks/use-toast'
import { getOrgSettingsFn, updateOrgSettingsFn, testSlackWebhookFn } from '@/api/server_functions'
import { ToastAction } from '@/components/ui/toast'


export const Route = createFileRoute(
  '/_authenticated/_dashboard/dashboard/drift',
)({
  component: RouteComponent,
  loader: async ({ context }) => {
    const { user, organisationId } = context;
    const settings = await getOrgSettingsFn({data: {userId: user?.id || '', organisationId: organisationId || ''}})
    return { settings, user, organisationId }
  }
})


function RouteComponent() {
  const { settings, user, organisationId } = Route.useLoaderData()
    const [settingsState, setSettingsState] = useState<OrgSettings>({
        drift_webhook_url: settings.drift_webhook_url,
        drift_enabled: settings.drift_enabled,
        drift_cron_tab: settings.drift_cron_tab  // Default to daily at 9 AM
    })
    const [saving, setSaving] = useState(false)
    const [testing, setTesting] = useState(false)
    const { toast } = useToast()


    const handleSave = async () => {
        try {
          setSaving(true)
          await updateOrgSettingsFn({data: {userId: user?.id || '', organisationId: organisationId || '', settings: settingsState}})
          toast({
            title: "Success",
            description: "Slack webhook settings saved successfully",
            action: <ToastAction altText="OK">OK</ToastAction>,
          })
        } catch (error) {
          console.error('Error saving Slack webhook settings:', error)
          toast({
            title: "Error",
            description: "Failed to save Slack webhook settings",
            variant: "destructive",
          })
        } finally {
          setSaving(false)
        }
      }
    
      const handleTest = async () => {
        if (!settingsState.drift_webhook_url) {
          toast({
            title: "Error",
            description: "Please enter a webhook URL first",
            variant: "destructive",
          })
          return
        }
    
        try {
          setTesting(true)
    
          await testSlackWebhookFn({data: {notification_url: settingsState.drift_webhook_url}})
          toast({
            title: "Success",
            description: "Test message sent successfully! Check your Slack channel.",
            action: <ToastAction altText="OK">OK</ToastAction>,
          })
        } catch (error) {
          console.error('Error testing Slack webhook:', error)
          toast({
            title: "Error",
            description: "Failed to send test message. Please check your webhook URL.",
            variant: "destructive",
          })
        } finally {
          setTesting(false)
        }
      }
    
      const isValidWebhookUrl = (url: string) => {
        return url.startsWith('https://hooks.slack.com/services/')
      }
    
      const cronPresets = [
        { label: "Hourly", value: "0 * * * *" },
        { label: "Daily", value: "0 9 * * *" },
        { label: "Every Sunday", value: "0 9 * * 0" },
        { label: "Weekly", value: "0 9 * * 1" },
        { label: "Monthly", value: "0 9 1 * *" }
      ]
    
      const isValidCrontab = (cron: string) => {
        // Basic validation for crontab format (5 fields)
        const fields = cron.trim().split(/\s+/)
        return fields.length === 5
      }

  return (
    <div className="container mx-auto p-4">
      <div className="mb-6">
        <Button variant="ghost" asChild>
          <Link to="/dashboard/repos">
            <ArrowLeft className="mr-2 h-4 w-4" /> Back to Dashboard
          </Link>
        </Button>
      </div>

      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold mb-2">Drift Settings</h1>
          <p className="text-gray-600">Configure notifications for infrastructure drift detection</p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Slack className="h-5 w-5" />
              Slack Notifications
            </CardTitle>
            <CardDescription>
              Get notified in Slack when drift is detected in your infrastructure
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            <div className="flex items-center justify-between">
              <div className="space-y-0.5">
                <Label className="text-base font-medium">Enable Slack notifications</Label>
                <div className="text-sm text-gray-600">
                  Receive drift reports and alerts in your Slack workspace
                </div>
              </div>
              <Switch
                checked={settingsState.drift_enabled}
                onCheckedChange={(checked) => 
                  {setSettingsState({ ...settingsState, drift_enabled: checked })}
                }
              />
            </div>

            <div className="space-y-3">
              <Label htmlFor="webhook-url" className="text-base font-medium">
                Slack Webhook URL
              </Label>
              <Input
                id="webhook-url"
                type="url"
                placeholder="https://hooks.slack.com/services/..."
                value={settingsState.drift_webhook_url}
                onChange={(e) => 
                  setSettingsState({ ...settingsState, drift_webhook_url: e.target.value })
                }
                className="font-mono"
              />
              <div className="text-sm text-gray-600">
                You can find this URL in your Slack app settings under "Incoming Webhooks"
              </div>
            </div>

            {settingsState.drift_webhook_url && !isValidWebhookUrl(settingsState.drift_webhook_url) && (
              <Alert>
                <AlertCircle className="h-4 w-4" />
                <AlertDescription>
                  Please enter a valid Slack webhook URL that starts with https://hooks.slack.com/services/
                </AlertDescription>
              </Alert>
            )}

            {settingsState.drift_webhook_url && isValidWebhookUrl(settingsState.drift_webhook_url) && (
              <Alert>
                <CheckCircle className="h-4 w-4" />
                <AlertDescription>
                  Webhook URL looks good! You can test it to make sure it's working.
                </AlertDescription>
              </Alert>
            )}

            <div className="flex gap-3 pt-4">
              <Button 
                variant="outline" 
                onClick={handleTest} 
                disabled={testing || !settingsState.drift_webhook_url || !isValidWebhookUrl(settingsState.drift_webhook_url)}
              >
                <TestTube className="mr-2 h-4 w-4" />
                {testing ? "Testing..." : "Test Webhook"}
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Clock className="h-5 w-5" />
              Notification Schedule
            </CardTitle>
            <CardDescription>
              Configure when to check for drift and send notifications
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            <div className="space-y-3">
              <Label className="text-base font-medium">Quick Presets</Label>
              <div className="flex flex-wrap gap-2">
                {cronPresets.map((preset) => (
                  <Button
                    key={preset.value}
                    variant={settingsState.drift_cron_tab === preset.value ? "default" : "outline"}
                    size="sm"
                    onClick={() => setSettingsState({ ...settingsState, drift_cron_tab: preset.value })}
                  >
                    {preset.label}
                  </Button>
                ))}
              </div>
            </div>

            <div className="space-y-3">
              <Label htmlFor="crontab" className="text-base font-medium">
                Custom Crontab Schedule
              </Label>
              <Input
                id="crontab"
                type="text"
                placeholder="0 9 * * *"
                value={settingsState.drift_cron_tab}
                onChange={(e) => 
                  setSettingsState({ ...settingsState, drift_cron_tab: e.target.value })
                }
                className="font-mono"
              />
              <div className="text-sm text-gray-600">
                Format: minute hour day month day-of-week (e.g., "0 9 * * *" for daily at 9 AM)
              </div>
            </div>

            {settingsState.drift_cron_tab && !isValidCrontab(settingsState.drift_cron_tab) && (
              <Alert>
                <AlertCircle className="h-4 w-4" />
                <AlertDescription>
                  Please enter a valid crontab format with 5 fields (minute hour day month day-of-week)
                </AlertDescription>
              </Alert>
            )}

            {settingsState.drift_cron_tab && isValidCrontab(settingsState.drift_cron_tab) && (
              <Alert>
                <CheckCircle className="h-4 w-4" />
                <AlertDescription>
                  Schedule format is valid. Drift checks will run according to this schedule.
                </AlertDescription>
              </Alert>
            )}

            <div className="flex gap-3 pt-4">
              <Button onClick={handleSave} disabled={saving}>
                <Save className="mr-2 h-4 w-4" />
                {saving ? "Saving..." : "Save All Settings"}
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>How to Set Up Slack Webhook</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-4 text-sm">
              <div>
                <h4 className="font-medium mb-2">Step 1: Create a Slack App</h4>
                <p className="text-gray-600">
                  Go to <a href="https://api.slack.com/apps" target="_blank" rel="noopener noreferrer" className="text-blue-600 hover:underline">https://api.slack.com/apps</a> and create a new app for your workspace.
                </p>
              </div>
              
              <div>
                <h4 className="font-medium mb-2">Step 2: Enable Incoming Webhooks</h4>
                <p className="text-gray-600">
                  In your app settings, go to "Incoming Webhooks" and turn on "Activate Incoming Webhooks".
                </p>
              </div>
              
              <div>
                <h4 className="font-medium mb-2">Step 3: Create a Webhook</h4>
                <p className="text-gray-600">
                  Click "Add New Webhook to Workspace" and select the channel where you want to receive notifications.
                </p>
              </div>
              
              <div>
                <h4 className="font-medium mb-2">Step 4: Copy the Webhook URL</h4>
                <p className="text-gray-600">
                  Copy the webhook URL provided by Slack and paste it in the field above.
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Crontab Schedule Examples</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-4 text-sm">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <h4 className="font-medium mb-2">Common Schedules</h4>
                  <div className="space-y-1 text-gray-600">
                    <div><code className="bg-gray-100 px-1 rounded">0 * * * *</code> - Every hour</div>
                    <div><code className="bg-gray-100 px-1 rounded">0 9 * * *</code> - Daily at 9 AM</div>
                    <div><code className="bg-gray-100 px-1 rounded">0 9 * * 0</code> - Every Sunday at 9 AM</div>
                    <div><code className="bg-gray-100 px-1 rounded">0 9 1 * *</code> - Monthly on the 1st at 9 AM</div>
                  </div>
                </div>
                <div>
                  <h4 className="font-medium mb-2">Crontab Format</h4>
                  <div className="space-y-1 text-gray-600">
                    <div><code className="bg-gray-100 px-1 rounded">*</code> - Any value</div>
                    <div><code className="bg-gray-100 px-1 rounded">*/5</code> - Every 5 units</div>
                    <div><code className="bg-gray-100 px-1 rounded">1-5</code> - Range from 1 to 5</div>
                    <div><code className="bg-gray-100 px-1 rounded">1,3,5</code> - Specific values</div>
                    <div className="text-xs mt-2">
                      Format: minute (0-59) hour (0-23) day (1-31) month (1-12) day-of-week (0-6)
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
  
}
