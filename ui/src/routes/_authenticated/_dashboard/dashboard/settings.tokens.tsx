import { createFileRoute } from '@tanstack/react-router'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useState } from 'react'

export const Route = createFileRoute(
  '/_authenticated/_dashboard/dashboard/settings/tokens',
)({
  component: RouteComponent,
})

function RouteComponent() {
  const [tokens, setTokens] = useState<string[]>([])
  const [newToken, setNewToken] = useState('')

  const generateToken = () => {
    // This is a placeholder - implement actual token generation logic
    const token = `digger_${Math.random().toString(36).substring(2)}`
    setTokens([...tokens, token])
    setNewToken(token)
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>API Tokens</CardTitle>
        <CardDescription>
          Generate and manage your API tokens for accessing Digger programmatically
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex space-x-4">
          <Button onClick={generateToken}>Generate New Token</Button>
        </div>
        {newToken && (
          <div className="space-y-2">
            <p className="text-sm font-medium">New Token (copy this now, it won't be shown again):</p>
            <div className="flex space-x-2">
              <Input value={newToken} readOnly />
              <Button variant="outline" onClick={() => navigator.clipboard.writeText(newToken)}>
                Copy
              </Button>
            </div>
          </div>
        )}
        <div className="space-y-2">
          <h4 className="text-sm font-medium">Your Tokens</h4>
          {tokens.length === 0 ? (
            <p className="text-sm text-muted-foreground">No tokens generated yet</p>
          ) : (
            <div className="space-y-2">
              {tokens.map((token, index) => (
                <div key={index} className="flex items-center justify-between">
                  <code className="text-sm">•••••••••••{token.slice(-4)}</code>
                  <Button
                    variant="destructive"
                    size="sm"
                    onClick={() => setTokens(tokens.filter((_, i) => i !== index))}
                  >
                    Revoke
                  </Button>
                </div>
              ))}
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
