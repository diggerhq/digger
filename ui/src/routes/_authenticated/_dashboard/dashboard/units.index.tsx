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
 
import { useEffect, useState } from "react"
import UnitCreateForm from "@/components/UnitCreateForm"
import { listUnitsFn } from '@/api/statesman_serverFunctions'
import { PageLoading } from '@/components/LoadingSkeleton'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { ChevronDown, X } from 'lucide-react'

export const Route = createFileRoute(
  '/_authenticated/_dashboard/dashboard/units/',
)({
  component: RouteComponent,
  pendingComponent: PageLoading,
  loader: async ({ context }) => {
    const { user, organisationId } = context;
    const pageSize = 20;
    const unitsData = await listUnitsFn({
      data: {
        organisationId: organisationId || '', 
        userId: user?.id || '', 
        email: user?.email || '',
        page: 1,
        pageSize,
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

function LoomBanner() {
  const [dismissed, setDismissed] = useState(false)
  const [open, setOpen] = useState(false)

  useEffect(() => {
    if (typeof window === 'undefined') return
    window.localStorage.setItem('units_loom_open', String(open))
  }, [open])

  const handleDismiss = () => {
    setDismissed(true)
    try {
      window.localStorage.setItem('units_loom_dismissed', 'true')
    } catch {}
  }

  if (dismissed) return null

  return (
    <div className="mb-4">
      <Collapsible open={open} onOpenChange={setOpen}>
        <div className="rounded-md border bg-muted/30">
          <div className="flex items-center justify-between px-4 py-3">
            <CollapsibleTrigger className="flex-1 flex items-center justify-between text-left">
              <span className="font-medium">Watch a quick walkthrough (2 min)</span>
              <ChevronDown className="h-4 w-4 transition-transform data-[state=open]:rotate-180" />
            </CollapsibleTrigger>
            <button
              className="ml-3 p-1 rounded hover:bg-muted"
              aria-label="Dismiss walkthrough"
              onClick={handleDismiss}
            >
              <X className="h-4 w-4" />
            </button>
          </div>
          <CollapsibleContent className="px-4 pb-4">
            <div className="relative pt-[56.25%]">
              <iframe
                src="https://www.loom.com/embed/0f303822db4147b1a0f89eeaa8df18ae"
                title="OpenTaco Units walkthrough"
                allow="autoplay; clipboard-write; encrypted-media; picture-in-picture"
                allowFullScreen
                className="absolute inset-0 h-full w-full rounded-md"
              />
            </div>
          </CollapsibleContent>
        </div>
      </Collapsible>
    </div>
  )
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
  const [pageData, setPageData] = useState(unitsData)
  const [currentPage, setCurrentPage] = useState(unitsData?.page || 1)
  const pageSize = (pageData as any)?.page_size || 20
  const total = (pageData as any)?.total || (pageData as any)?.units?.length || 0
  const units = (pageData?.units || []).slice().sort((a: any, b: any) => a.name.localeCompare(b.name))
  const navigate = Route.useNavigate()
  const router = useRouter()

  async function loadPage(page: number) {
    const next = await listUnitsFn({
      data: {
        organisationId: organisationId || '',
        userId: user?.id || '',
        email: user?.email || '',
        page,
        pageSize,
      },
    })
    setPageData(next)
    setCurrentPage(next?.page || page)
  }
  
  // Handle optimistic update - add immediately
  function handleUnitOptimistic(tempUnit: any) {
    setPageData(prev => {
      const nextUnits = [{
        ...tempUnit,
        locked: false,
        size: 0,
        updated: new Date(),
        isOptimistic: true
      }, ...(prev?.units || [])]
      return { ...prev, units: nextUnits }
    })
  }
  
  // Handle actual creation - refresh from server
  async function handleUnitCreated() {
    await loadPage(1)
  }
  
  // Handle failure - remove optimistic unit
  function handleUnitFailed() {
    setPageData(prev => ({ ...prev, units: (prev?.units || []).filter((u: any) => !u.isOptimistic) }))
  }
  
  const canGoPrev = currentPage > 1
  const canGoNext = currentPage * pageSize < total

  return (<>
    <div className="container mx-auto p-4">
      <div className="mb-6">
        <Button variant="ghost" asChild>
          <Link to="/dashboard/repos">
            <ArrowLeft className="mr-2 h-4 w-4" /> Back to Dashboard
          </Link>
        </Button>
      </div>
      
      {/* Loom walkthrough banner - collapsible and dismissible */}
      { units.length === 0 && <LoomBanner /> }

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
      <div className="flex items-center justify-between pt-3">
        <p className="text-sm text-muted-foreground">
          Showing {units.length} of {total} units (page {currentPage}, {pageSize} per page)
        </p>
        <div className="space-x-2">
          <Button variant="outline" size="sm" disabled={!canGoPrev} onClick={() => loadPage(currentPage - 1)}>Previous</Button>
          <Button variant="outline" size="sm" disabled={!canGoNext} onClick={() => loadPage(currentPage + 1)}>Next</Button>
        </div>
      </div>
        </CardContent>
      </Card>
    </div>
</>)
}
