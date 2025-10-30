import { createFileRoute } from '@tanstack/react-router'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useState } from 'react'
import { createTokenFn, deleteTokenFn, getTokensFn } from '@/api/tokens_serverFunctions'
import { useToast } from '@/hooks/use-toast'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

export const Route = createFileRoute(
  '/_authenticated/_dashboard/dashboard/settings/tokens',
)({
  component: RouteComponent,
  loader: async ({ context }) => {
    const { user, organisationId } = context;
    const tokens = await getTokensFn({data: {organizationId: organisationId, userId: user?.id || ''}})
    return { tokens, user, organisationId }
  }
})

function RouteComponent() {
  const { tokens, user, organisationId } = Route.useLoaderData()
  const [tokenList, setTokenList] = useState<typeof tokens>(tokens)
  const [newToken, setNewToken] = useState('')
  const [open, setOpen] = useState(false)
  const [nickname, setNickname] = useState('')
  const [expiry, setExpiry] = useState<'1_week' | '30_days' | 'no_expiry'>('1_week')
  const [submitting, setSubmitting] = useState(false)
  const { toast } = useToast()
  const computeExpiry = (value: '1_week' | '30_days' | 'no_expiry'): string | null => {
    console.log('value', value)
    if (value === 'no_expiry') return null
    if (value === '1_week') return `${7*24}h`
    if (value === '30_days') return `${30*24}h`
    return `${7*24}h`
  }

  function formatDateString(value?: string | null) {
    if (!value) return '—'
    const d = new Date(value)
    if (isNaN(d.getTime())) return String(value)
    return d.toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    })
  }

  function isTokenExpired(token: any) {
    if (token?.status && token.status !== 'active') return true
    if (token?.expires_at) {
      const exp = new Date(token.expires_at)
      if (!isNaN(exp.getTime()) && exp.getTime() < Date.now()) return true
    }
    return false
  }

  const onConfirmGenerate = async () => {
    setSubmitting(true)
    try {
      const expiresAt = computeExpiry(expiry)
      const created = await createTokenFn({data: {organizationId: organisationId, userId: user?.id || '', name: nickname || 'New Token', expiresAt}})
      if (created && created.token) {
        setNewToken(created.token)
      }
      setOpen(false)
      setNickname('')
      setExpiry('no_expiry')
      const newTokenList = await getTokensFn({data: {organizationId: organisationId, userId: user?.id || ''}})
      setTokenList(newTokenList)
    } finally {
      setSubmitting(false)
    }
  }

  const handleRevokeToken = async (tokenId: string) => {
    deleteTokenFn({data: {organizationId: organisationId, userId: user?.id || '', tokenId: tokenId}}).then(() => {
      toast({
        title: 'Token revoked',
        description: 'The token has been revoked',
      })
    }).catch((error) => {
      toast({
        title: 'Failed to revoke token',
        description: error.message,
        variant: 'destructive',
      })
    }).finally(async () => {
      setSubmitting(false)
      const newTokenList = await getTokensFn({data: {organizationId: organisationId, userId: user?.id || ''}})
      setTokenList(newTokenList)
    })
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
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
              <Button>Generate New Token</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Generate API Token</DialogTitle>
                <DialogDescription>Provide a nickname and choose an expiry.</DialogDescription>
              </DialogHeader>
              <div className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="nickname">Nickname</Label>
                  <Input id="nickname" placeholder="e.g. CI token" value={nickname} onChange={(e) => setNickname(e.target.value)} />
                </div>
                <div className="space-y-2">
                  <Label>Expiry</Label>
                  <Select value={expiry} onValueChange={(v) => setExpiry(v as typeof expiry)}>
                    <SelectTrigger>
                      <SelectValue placeholder="Select expiry" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="1_week">1 week</SelectItem>
                      <SelectItem value="30_days">30 days</SelectItem>
                      <SelectItem value="no_expiry">No expiry</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setOpen(false)} disabled={submitting}>Cancel</Button>
                <Button onClick={onConfirmGenerate} disabled={submitting || (!nickname && expiry === 'no_expiry')}>{submitting ? 'Generating...' : 'Generate'}</Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>
        {newToken && (
          <div className="space-y-2 bg-blue-50 p-4 rounded-lg border border-blue-200">
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
          {tokenList.length === 0 ? (
            <p className="text-sm text-muted-foreground">No tokens generated yet</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="text-left">Name</TableHead>
                  <TableHead className="text-left">Token</TableHead>
                  <TableHead className="text-left">Expires</TableHead>
                  <TableHead className="text-left">Created</TableHead>
                  <TableHead className="text-left">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {tokenList.map((token, index) => (
                  <TableRow key={index}>
                    <TableCell className="font-medium">{token.name}</TableCell>
                    <TableCell>•••••••••••{token.token.slice(-4)}</TableCell>
                    <TableCell>
                      {isTokenExpired(token)
                        ? <span className="text-destructive">This token has expired</span>
                        : (token.expires_at ? formatDateString(token.expires_at) : 'No expiry')}
                    </TableCell>
                    <TableCell>{formatDateString(token.created_at)}</TableCell>
                    <TableCell>
                      <Button
                        variant="destructive"
                        size="sm"
                        onClick={() => handleRevokeToken(token.id)}
                      >
                        Revoke
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
