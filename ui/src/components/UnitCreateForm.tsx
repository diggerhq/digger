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
  const [engine, setEngine] = React.useState<'terraform' | 'tofu'>('terraform')
  const [terraformVersion, setTerraformVersion] = React.useState('1.5.7')
  const [isCreating, setIsCreating] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)
  
  // Remote runs are gated by localStorage flag for beta testing
  // Set localStorage.setItem('REMOTE_RUNS', 'true') to enable
  const remoteRunsEnabled = typeof window !== 'undefined' && localStorage.getItem('REMOTE_RUNS') === 'true'

  const handleCreate = async () => {
    if (!unitName.trim()) return
    setIsCreating(true)
    setError(null)
    
    const tempId = `temp-${Date.now()}`
    const tempUnit = { id: tempId, name: unitName.trim(), isOptimistic: true }
    const finalUnitType = remoteRunsEnabled ? unitType : 'local'
    
    // Optimistic update - show immediately
    if (onCreatedOptimistic) {
      onCreatedOptimistic(tempUnit)
    }
    
    try {
      const finalUnitType = remoteRunsEnabled ? unitType : 'local'
      const unit = await createUnitFn({
        data: {
          userId,
          organisationId,
          email,
          name: unitName.trim(),
          // Enable TFE remote execution for remote type
          // Auto-apply defaults to false - user must explicitly approve applies
          tfeAutoApply: false,
          tfeExecutionMode: finalUnitType === 'remote' ? 'remote' : 'local',
          tfeTerraformVersion: finalUnitType === 'remote' ? terraformVersion : undefined,
          tfeEngine: finalUnitType === 'remote' ? engine : undefined,
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
            className={`relative flex items-start gap-4 rounded-lg border p-4 md:p-5 transition-colors ${
              remoteRunsEnabled
                ? 'cursor-pointer hover:bg-muted/50'
                : 'cursor-not-allowed opacity-60'
            } ${unitType === 'remote' ? 'ring-2 ring-primary border-primary' : 'border-muted'}`}
            onClick={() => {
              if (remoteRunsEnabled) {
                setUnitType('remote')
              }
            }}
          >
            <RadioGroupItem
              id="unit-type-remote"
              value="remote"
              className="sr-only"
              disabled={!remoteRunsEnabled}
            />
            <div className="flex h-10 w-10 items-center justify-center rounded-md bg-primary/10 text-primary">
              <Cloud className="h-5 w-5" />
            </div>
            <div className="space-y-1">
              <div className="flex items-center gap-2">
                <span className="text-base font-semibold">Remote</span>
                <Badge variant="secondary">
                  {remoteRunsEnabled ? 'Beta' : 'Coming soon'}
                </Badge>
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

      {unitType === 'remote' && (
        <>
          <div>
            <Label htmlFor="engine">Engine</Label>
            <div className="mt-2">
              <RadioGroup
                value={engine}
                onValueChange={(v) => {
                  setEngine(v as 'terraform' | 'tofu')
                  // Set default version based on engine
                  if (v === 'tofu') {
                    setTerraformVersion('1.10.0')
                  } else {
                    setTerraformVersion('1.5.7')
                  }
                }}
                className="flex gap-4"
              >
                <label
                  htmlFor="engine-terraform"
                  className={`flex-1 cursor-pointer rounded-lg border p-4 transition-colors hover:bg-muted/50 ${engine === 'terraform' ? 'ring-2 ring-primary border-primary' : 'border-muted'}`}
                  onClick={() => {
                    setEngine('terraform')
                    setTerraformVersion('1.5.7')
                  }}
                >
                  <RadioGroupItem id="engine-terraform" value="terraform" className="sr-only" />
                  <div className="font-semibold">Terraform</div>
                  <div className="text-xs text-muted-foreground">HashiCorp Terraform</div>
                </label>
                <label
                  htmlFor="engine-tofu"
                  className={`flex-1 cursor-pointer rounded-lg border p-4 transition-colors hover:bg-muted/50 ${engine === 'tofu' ? 'ring-2 ring-primary border-primary' : 'border-muted'}`}
                  onClick={() => {
                    setEngine('tofu')
                    setTerraformVersion('1.10.0')
                  }}
                >
                  <RadioGroupItem id="engine-tofu" value="tofu" className="sr-only" />
                  <div className="font-semibold">OpenTofu</div>
                  <div className="text-xs text-muted-foreground">Open-source fork</div>
                </label>
              </RadioGroup>
            </div>
          </div>
          
          <div>
            <Label htmlFor="terraform-version">{engine === 'tofu' ? 'OpenTofu' : 'Terraform'} Version</Label>
            <div className="mt-2 space-y-2">
              <Input
                id="terraform-version"
                value={terraformVersion}
                onChange={(e) => setTerraformVersion(e.target.value)}
                placeholder={engine === 'tofu' ? '1.10.0' : '1.5.7'}
                className="font-mono"
              />
              {engine === 'terraform' && !!terraformVersion && parseFloat(terraformVersion) >= 1.6 && (
                <div className="flex items-start gap-2 rounded-md bg-yellow-50 dark:bg-yellow-950 p-3 text-sm text-yellow-800 dark:text-yellow-200">
                  <svg className="h-5 w-5 flex-shrink-0 mt-0.5" fill="currentColor" viewBox="0 0 20 20">
                    <path fillRule="evenodd" d="M8.485 2.495c.673-1.167 2.357-1.167 3.03 0l6.28 10.875c.673 1.167-.17 2.625-1.516 2.625H3.72c-1.347 0-2.189-1.458-1.515-2.625L8.485 2.495zM10 5a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0v-3.5A.75.75 0 0110 5zm0 9a1 1 0 100-2 1 1 0 000 2z" clipRule="evenodd" />
                  </svg>
                  <div>
                    <strong className="font-semibold">Warning: Unsupported version</strong>
                    <p className="mt-1">Terraform versions 1.6.0 and above are not officially supported.</p>
                  </div>
                </div>
              )}
              <p className="text-xs text-muted-foreground">
                {engine === 'terraform' ? (
                  <>Pre-built versions: 1.0.11, 1.3.9, 1.5.7 (fast startup). Custom versions installed at runtime. We do not support versions above 1.5.7</>
                ) : (
                  <>Pre-built versions: 1.6.0, 1.10.0 (fast startup). Custom versions installed at runtime.</>
                )}
              </p>
            </div>
          </div>
        </>
      )}

      {error && <p className="text-sm text-destructive">{error}</p>}
      {showBringOwnState ? (
        <div className="flex items-center justify-between">
          <Button variant="ghost" type="button" onClick={onBringOwnState}>
            I want to bring my own state
          </Button>
          <Button 
            onClick={handleCreate} 
            disabled={
              !unitName.trim() || 
              isCreating || 
              (engine === 'terraform' && !!terraformVersion && parseFloat(terraformVersion) >= 1.6)
            }
          >
            {isCreating ? 'Creating...' : 'Create Unit'}
          </Button>
        </div>
      ) : (
        <div className="flex items-center justify-end">
          <Button 
            onClick={handleCreate} 
            disabled={
              !unitName.trim() || 
              isCreating || 
              (engine === 'terraform' && !!terraformVersion && parseFloat(terraformVersion) >= 1.6)
            }
          >
            {isCreating ? 'Creating...' : 'Create Unit'}
          </Button>
        </div>
      )}
    </div>
  )
}


