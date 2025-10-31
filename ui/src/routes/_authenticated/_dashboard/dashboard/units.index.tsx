import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { createFileRoute, Link, useRouter } from '@tanstack/react-router'
import { ArrowLeft, Plus, Database, ExternalLink, Lock, Unlock, Cloud, HardDrive } from 'lucide-react'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Badge } from "@/components/ui/badge"
import { useState } from "react"
import { createUnitFn } from "@/api/statesman_serverFunctions"
import { listUnitsFn } from '@/api/statesman_serverFunctions'

export const Route = createFileRoute(
  '/_authenticated/_dashboard/dashboard/units/',
)({
  component: RouteComponent,
  loader: async ({ context }) => {
    const { user, organisationId } = context;
    const unitsData = await listUnitsFn({data: {organisationId: organisationId || '', userId: user?.id || '', email: user?.email || ''}})
    return { unitsData: unitsData, user, organisationId } 
  }
})

function formatBytes(bytes: number) {
  if (bytes === 0) return '0 Bytes'
  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function formatDate(date: Date) {
  return date.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

function CreateUnitModal({ onUnitCreated }: { onUnitCreated: () => void }) {
  const [open, setOpen] = useState(false)
  const [unitName, setUnitName] = useState("")
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [unitType, setUnitType] = useState<'local' | 'remote'>("local")
  const { user, organisationId } = Route.useLoaderData()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsLoading(true)
    setError(null)

    try {
      const unit = await createUnitFn({
        data: {
          userId: user?.id!,
          organisationId,
          email: user?.email || '',
          name: unitName,
        }
      })
      setOpen(false)
      setUnitName("")
      onUnitCreated()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create unit")
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button>
          <Plus className="mr-2 h-4 w-4" />
          Create New Unit
        </Button>
      </DialogTrigger>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Create New Unit</DialogTitle>
            <DialogDescription>
              Enter a name for your new terraform state unit.
            </DialogDescription>
          </DialogHeader>
          <div className="py-4">
            <Label htmlFor="name" className="text-right">
              Unit Name
            </Label>
            <Input
              id="name"
              value={unitName}
              onChange={(e) => setUnitName(e.target.value)}
              placeholder="my-terraform-state"
              className="mt-1"
            />
            {error && <p className="text-sm text-destructive mt-2">{error}</p>}
          </div>
          <div className="py-2">
            <Label className="text-right">Unit Type</Label>
            <RadioGroup
              value={unitType}
              onValueChange={(v) => setUnitType(v as 'local' | 'remote')}
              className="mt-3 grid grid-cols-1 gap-3"
            >
              {/* Local option card */}
              <label
                htmlFor="unit-type-local"
                className={
                  `relative flex cursor-pointer items-start gap-4 rounded-lg border p-4 md:p-5 transition-colors hover:bg-muted/50 ` +
                  (unitType === 'local' ? 'ring-2 ring-primary border-primary' : 'border-muted')
                }
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

              {/* Remote option card (disabled) */}
              <label
                htmlFor="unit-type-remote"
                className={
                  `relative flex cursor-not-allowed items-start gap-4 rounded-lg border p-4 md:p-5 opacity-60 ` +
                  `bg-muted/30`
                }
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
          <DialogFooter>
            <Button
              type="submit"
              disabled={!unitName.trim() || isLoading}
            >
              {isLoading ? "Creating..." : "Create Unit"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function RouteComponent() {
  const {  unitsData, organisationId, user } = Route.useLoaderData()
  const [units, setUnits] = useState(unitsData?.units || [])
  const navigate = Route.useNavigate()
  const router = useRouter()
  
  async function handleUnitCreated() {
    const unitsData = await listUnitsFn({data: {organisationId: organisationId, userId: user?.id || '', email: user?.email || ''}})
    setUnits(unitsData.units)
  }
  
  return (<>
    <div className="container mx-auto p-4">
      <div className="mb-6">
        <Button variant="ghost" asChild>
          <Link to="/dashboard/repos">
            <ArrowLeft className="mr-2 h-4 w-4" /> Back to Dashboard
          </Link>
        </Button>
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <div>
            <CardTitle>Units</CardTitle>
            <CardDescription>List of terraform state units and their current status</CardDescription>
          </div>
          <CreateUnitModal onUnitCreated={handleUnitCreated} />
        </CardHeader>
        <CardContent>
          {units.length === 0 ? (
        <div className="text-center py-12">
          <div className="inline-flex h-12 w-12 items-center justify-center rounded-full bg-primary/10 mb-4">
            <Database className="h-6 w-6 text-primary" />
          </div>
          <h2 className="text-lg font-semibold mb-2">No Units Created Yet</h2>
          <p className="text-muted-foreground max-w-sm mx-auto mb-6">
            Units are equivalent to individual terraform statefiles - but they also include version history by default and you can rollback to a previous version of a unit's state.
          </p>
          <CreateUnitModal onUnitCreated={() => navigate({ to: '/dashboard/units' })} />
        </div>
      ) : (
        <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Status</TableHead>
            <TableHead>Name</TableHead>
            <TableHead>Size</TableHead>
            <TableHead>Last Updated</TableHead>
            <TableHead></TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {units.map((unit) => (
            <TableRow key={unit.id}>
              <TableCell>
                {unit.locked ? <Lock className="h-5 w-5 text-destructive" /> : <Unlock className="h-5 w-5 text-muted-foreground" />}
              </TableCell>
              <TableCell className="font-medium">{unit.name}</TableCell>
              <TableCell>{formatBytes(unit.size)}</TableCell>
              <TableCell>{formatDate(unit.updatedAt || new Date())}</TableCell>
              <TableCell className="text-right">
                <Button variant="ghost" asChild className="justify-end">
                  <Link to={`/dashboard/units/$unitId`} params={{ unitId: unit.id }}>
                    View Details <ExternalLink className="ml-2 h-4 w-4" />
                  </Link>
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      )}
        </CardContent>
      </Card>
    </div>
</>)
}
