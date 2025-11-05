import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { createFileRoute, Link, useRouter } from '@tanstack/react-router'
import { ArrowLeft, Plus, Database, ExternalLink, Lock, Unlock } from 'lucide-react'
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
 
import { useState } from "react"
import UnitCreateForm from "@/components/UnitCreateForm"
import { listUnitsFn } from '@/api/statesman_serverFunctions'
import { PageLoading } from '@/components/LoadingSkeleton'

export const Route = createFileRoute(
  '/_authenticated/_dashboard/dashboard/units/',
)({
  component: RouteComponent,
  pendingComponent: PageLoading,
  loader: async ({ context }) => {
    const { user, organisationId } = context;
    
    const unitsData = await listUnitsFn({
      data: {
        organisationId: organisationId || '', 
        userId: user?.id || '', 
        email: user?.email || ''
      }
    });
    
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

function formatDate(value: any) {
  if (!value) return '—'
  const d = value instanceof Date ? value : new Date(value)
  if (isNaN(d.getTime())) return '—'
  return d.toLocaleString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

function CreateUnitModal({ onUnitCreated, onUnitOptimistic, onUnitFailed }: { 
  onUnitCreated: () => void,
  onUnitOptimistic: (unit: any) => void,
  onUnitFailed: () => void
}) {
  const [open, setOpen] = useState(false)
  const { user, organisationId } = Route.useLoaderData()
  const navigate = Route.useNavigate()

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button>
          <Plus className="mr-2 h-4 w-4" />
          Create New Unit
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create New Unit</DialogTitle>
          <DialogDescription>
            Enter a name for your new terraform state unit.
          </DialogDescription>
        </DialogHeader>
        <div className="py-2">
          <UnitCreateForm
            userId={user?.id || ''}
            email={user?.email || ''}
            organisationId={organisationId}
            onCreatedOptimistic={(tempUnit) => {
              onUnitOptimistic(tempUnit)
              setOpen(false)
            }}
            onCreated={() => { 
              setOpen(false)
              onUnitCreated()
            }}
            onCreatedFailed={() => {
              onUnitFailed()
            }}
            onBringOwnState={() => { setOpen(false); navigate({ to: '/dashboard/onboarding' }); }}
            showBringOwnState={false}
          />
        </div>
      </DialogContent>
    </Dialog>
  )
}

function RouteComponent() {
  const {  unitsData, organisationId, user } = Route.useLoaderData()
  const [units, setUnits] = useState(unitsData?.units || [])
  const navigate = Route.useNavigate()
  const router = useRouter()
  
  // Handle optimistic update - add immediately
  function handleUnitOptimistic(tempUnit: any) {
    setUnits(prev => [{
      ...tempUnit,
      locked: false,
      size: 0,
      updated: new Date(),
      isOptimistic: true
    }, ...prev])
  }
  
  // Handle actual creation - refresh from server
  async function handleUnitCreated() {
    const unitsData = await listUnitsFn({data: {organisationId: organisationId, userId: user?.id || '', email: user?.email || ''}})
    setUnits(unitsData.units)
  }
  
  // Handle failure - remove optimistic unit
  function handleUnitFailed() {
    setUnits(prev => prev.filter((u: any) => !u.isOptimistic))
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
            <CardDescription className="mt-2">List of terraform state units and their current status</CardDescription>
          </div>
          
          {units.length > 0 && (<div className="flex items-center gap-2">
            <Button variant="outline" asChild>
              <Link to="/dashboard/onboarding">Show onboarding flow</Link>
            </Button>
            <CreateUnitModal 
              onUnitOptimistic={handleUnitOptimistic}
              onUnitCreated={handleUnitCreated} 
              onUnitFailed={handleUnitFailed}
            />
          </div>)}
        </CardHeader>
        <CardContent>
          {units.length === 0 ? (
        <div className="text-center py-12">
          <div className="inline-flex h-12 w-12 items-center justify-center rounded-full bg-primary/10 mb-4">
            <Database className="h-6 w-6 text-primary" />
          </div>
          <h2 className="text-lg font-semibold mb-2">No Units Created Yet</h2>
          <p className="text-muted-foreground max-w-sm mx-auto mb-6">
            Units are equivalent to individual terraform deployable pieces - but they also include version history by default and you can rollback to a previous version of a unit's state.
          </p>
          <div className="flex items-center justify-center gap-2">
            <Button asChild>
              <Link to="/dashboard/onboarding">Create first unit</Link>
            </Button>
          </div>
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
          {units.map((unit: any) => (
            <TableRow key={unit.id} className={unit.isOptimistic ? 'opacity-60' : ''}>
              <TableCell>
                {unit.locked ? <Lock className="h-5 w-5 text-destructive" /> : <Unlock className="h-5 w-5 text-muted-foreground" />}
              </TableCell>
              <TableCell className="font-medium">
                {unit.name}
                {unit.isOptimistic && <span className="ml-2 text-xs text-muted-foreground">(Creating...)</span>}
              </TableCell>
              <TableCell>{formatBytes(unit.size)}</TableCell>
              <TableCell>{formatDate(unit.updated)}</TableCell>
              <TableCell className="text-right">
                {!unit.isOptimistic && (
                  <Button variant="ghost" asChild className="justify-end">
                    <Link to={`/dashboard/units/$unitId`} params={{ unitId: unit.id }}>
                      View Details <ExternalLink className="ml-2 h-4 w-4" />
                    </Link>
                  </Button>
                )}
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
