import * as React from 'react'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { createUnitFn } from '@/api/statesman_serverFunctions'
import { Cloud, HardDrive } from 'lucide-react'

type UnitCreateFormProps = {
  userId: string
  email: string
  organisationId: string
  onCreated: (unit: { id: string; name: string }) => void
  onCreatedOptimistic?: (tempUnit: { id: string; name: string; isOptimistic: boolean }) => void
  onCreatedFailed?: () => void
  onBringOwnState: () => void
  showBringOwnState?: boolean
}

export default function UnitCreateForm({ 
  userId, 
  email, 
  organisationId, 
  onCreated, 
  onCreatedOptimistic,
  onCreatedFailed,
  onBringOwnState, 
  showBringOwnState = true 
}: UnitCreateFormProps) {
  const [unitName, setUnitName] = React.useState('')
  const [unitType, setUnitType] = React.useState<'local' | 'remote'>('local')
  const [isCreating, setIsCreating] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)

  const handleCreate = async () => {
    if (!unitName.trim()) return
    setIsCreating(true)
    setError(null)
    
    const tempId = `temp-${Date.now()}`
    const tempUnit = { id: tempId, name: unitName.trim(), isOptimistic: true }
    
    // Optimistic update - show immediately
    if (onCreatedOptimistic) {
      onCreatedOptimistic(tempUnit)
    }
    
    try {
      const unit = await createUnitFn({
        data: {
          userId,
          organisationId,
          email,
          name: unitName.trim(),
        },
      })
      // analytics: track unit creation
      try {
        const user = { id: userId, email }
        const { trackUnitCreated } = await import('@/lib/analytics')
        trackUnitCreated(user, organisationId, { id: unit.id, name: unit.name })
      } catch {}
      onCreated({ id: unit.id, name: unit.name })
    } catch (e: any) {
      setError(e?.message ?? 'Failed to create unit')
      if (onCreatedFailed) {
        onCreatedFailed()
      }
    } finally {
      setIsCreating(false)
    }
  }

  return (
    <div className="space-y-4">
      <div>
        <Label htmlFor="unit-name">Unit Name</Label>
        <Input
          id="unit-name"
          value={unitName}
          onChange={(e) => setUnitName(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              handleCreate()
            }
          }}
          placeholder="my-terraform-state"
          className="mt-1"
        />
      </div>

      <div>
        <Label className="text-right">Unit Type</Label>
        <RadioGroup
          value={unitType}
          onValueChange={(v) => setUnitType(v as 'local' | 'remote')}
          className="mt-3 grid grid-cols-1 gap-3"
        >
          <label
            htmlFor="unit-type-local"
            className={`relative flex cursor-pointer items-start gap-4 rounded-lg border p-4 md:p-5 transition-colors hover:bg-muted/50 ${unitType === 'local' ? 'ring-2 ring-primary border-primary' : 'border-muted'}`}
            onClick={() => setUnitType('local')}
          >
            <RadioGroupItem id="unit-type-local" value="local" className="sr-only" />
            <div className="flex h-10 w-10 items-center justify-center rounded-md bg-primary/10 text-primary">
              <HardDrive className="h-5 w-5" />
            </div>
            <div className="space-y-1">
              <div className="flex items-center gap-2">
                <span className="text-base font-semibold">Local</span>
              </div>
              <p className="text-sm text-muted-foreground">
                Mainly using units as states manager. Useful for teams that want full
                control and integrating pipelines with own CI.
              </p>
            </div>
          </label>

          <label
            htmlFor="unit-type-remote"
            className={`relative flex cursor-not-allowed items-start gap-4 rounded-lg border p-4 md:p-5 opacity-60 bg-muted/30`}
          >
            <RadioGroupItem id="unit-type-remote" value="remote" disabled className="sr-only" />
            <div className="flex h-10 w-10 items-center justify-center rounded-md bg-muted text-muted-foreground">
              <Cloud className="h-5 w-5" />
            </div>
            <div className="space-y-1">
              <div className="flex items-center gap-2">
                <span className="text-base font-semibold">Remote</span>
                <Badge variant="secondary">Coming soon</Badge>
              </div>
              <p className="text-sm text-muted-foreground">
                Fully managed terraform runs. Run terraform locally and stream logs from
                remote runs. Best for teams that want seamless automation for their
                terraform runs without much configuration.
              </p>
            </div>
          </label>
        </RadioGroup>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}
      {showBringOwnState ? (
        <div className="flex items-center justify-between">
          <Button variant="ghost" type="button" onClick={onBringOwnState}>
            I want to bring my own state
          </Button>
          <Button onClick={handleCreate} disabled={!unitName.trim() || isCreating}>
            {isCreating ? 'Creating...' : 'Create Unit'}
          </Button>
        </div>
      ) : (
        <div className="flex items-center justify-end">
          <Button onClick={handleCreate} disabled={!unitName.trim() || isCreating}>
            {isCreating ? 'Creating...' : 'Create Unit'}
          </Button>
        </div>
      )}
    </div>
  )
}


