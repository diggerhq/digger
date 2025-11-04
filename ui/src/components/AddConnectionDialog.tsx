"use client"

import { useState } from "react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Label } from "@/components/ui/label"
import { Input } from "@/components/ui/input"
import { Github } from "lucide-react"
import { GithubConnectButton } from "./GithubConnectButton"

interface AddConnectionDialogProps {
  onBitbucketSubmit: (data: { type: string; bitbucket_webhook_secret: string; bitbucket_access_token: string }) => Promise<void>
  onGitlabSubmit: (data: { type: string; gitlab_webhook_secret: string, gitlab_access_token: string }) => Promise<void>
  githubAppUrl: string
}

export function AddConnectionDialog({ onBitbucketSubmit, onGitlabSubmit, githubAppUrl  }: AddConnectionDialogProps) {
  const [connectionType, setConnectionType] = useState("")
  const [webhookSecret, setWebhookSecret] = useState("")
  const [bitbucketAccessToken, setBitbucketAccessToken] = useState("")
  const [gitlabAccessToken, setGitlabAccessToken] = useState("")

  const handleBitbucketSubmit = () => {
    onBitbucketSubmit({
      type: connectionType,
      bitbucket_webhook_secret: webhookSecret,
      bitbucket_access_token: bitbucketAccessToken
    })
  }

  const handleGitlabSubmit = () => {
    onGitlabSubmit({
      type: connectionType,
      gitlab_webhook_secret: webhookSecret,
      gitlab_access_token: gitlabAccessToken
    })
  }

  const handleGitHubConnect = async () => {
    window.location.href = githubAppUrl;
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Add New Connection</CardTitle>
        <CardDescription>Choose the type of connection you want to add. Webhook secret will only be shown once. You need to save it in order to enter the same in your repository webhook settings</CardDescription>
      </CardHeader>
      <CardContent>
        <RadioGroup value={connectionType} onValueChange={setConnectionType} className="space-y-4">
          <div className="flex items-center space-x-2">
            <RadioGroupItem value="github" id="github" />
            <Label htmlFor="github">GitHub</Label>
          </div>
          <div className="flex items-center space-x-2">
            <RadioGroupItem value="bitbucket" id="bitbucket" />
            <Label htmlFor="bitbucket">Bitbucket</Label> 
          </div>
          <div className="flex items-center space-x-2">
            <RadioGroupItem value="gitlab" id="gitlab" />
            <Label htmlFor="gitlab">GitLab</Label>
          </div>
        </RadioGroup>

        {connectionType === "github" && (
          <div className="mt-4 space-y-4">
            <p>To connect your GitHub repository, you need to install our <a href={githubAppUrl} target="_blank" rel="noopener noreferrer">GitHub App</a>.</p>
            <GithubConnectButton source="add_connection_dialog" onClick={handleGitHubConnect} />
          </div>
        )}

        {(connectionType === "bitbucket") && (
          <div className="mt-4 space-y-4">
            <div>
              <Label htmlFor="webhook-url">Webhook Secret</Label>
              <Input id="webhook-secret" placeholder={`Enter ${connectionType} webhook secret that you will use in the repo settings`} value={webhookSecret} onChange={(e) => setWebhookSecret(e.target.value)}/>
            </div>
            <div>
              <Label htmlFor="access-token">Access Token</Label>
              <Input id="access-token" type="password" placeholder={`Enter your ${connectionType} access token`} value={bitbucketAccessToken} onChange={(e) => setBitbucketAccessToken(e.target.value)} />
            </div>
            <Button type="submit" disabled={!connectionType} onClick={handleBitbucketSubmit}>
              Add Connection
            </Button>
          </div>
        )}


        {(connectionType === "gitlab") && (
          <div className="mt-4 space-y-4">
            <div>
              <Label htmlFor="webhook-url">Webhook Secret</Label>
              <Input id="webhook-secret" placeholder={`Enter ${connectionType} webhook secret that you will use in the repo settings`} value={webhookSecret} onChange={(e) => setWebhookSecret(e.target.value)}/>
            </div>
            <div>
              <Label htmlFor="access-token">Access Token</Label>
              <Input id="access-token" type="password" placeholder={`Enter your ${connectionType} access token`} value={gitlabAccessToken} onChange={(e) => setGitlabAccessToken(e.target.value)} />
            </div>
            <Button type="submit" disabled={!connectionType} onClick={handleGitlabSubmit}>
              Add Connection
            </Button>
          </div>
        )}

      </CardContent>
      <CardFooter />
    </Card>
  )
}

