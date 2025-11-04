import { createFileRoute, Link, useParams, useRouter } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs"
import { Badge } from "@/components/ui/badge"
import { ArrowLeft, Lock, Unlock, MoreVertical, History, Trash2, Download, Upload, RefreshCcw, Copy, Check, ArrowUpRight } from 'lucide-react'
import { useState } from 'react'
import { toast } from '@/hooks/use-toast'
import { getUnitFn, getUnitVersionsFn, lockUnitFn, unlockUnitFn, getUnitStatusFn, deleteUnitFn, downloadLatestStateFn, restoreUnitStateVersionFn } from '@/api/statesman_serverFunctions'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { getPublicServerConfig } from '@/lib/env.server'
import { DetailsSkeleton } from '@/components/LoadingSkeleton'
import type { Env } from '@/lib/env.server'
import { downloadJson } from '@/lib/io'

import { 
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import UnitStateForceUploadDialog from '@/components/UnitStateForceUploadDialog'
import UnitConfigureInstructions from '@/components/UnitConfigureInstructions'

function CopyButton({ content }: { content: string }) {
  const [copied, setCopied] = useState(false)

  const copy = () => {
    navigator.clipboard.writeText(content)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <Button
      size="icon"
      variant="ghost"
      className="absolute top-2 right-2 h-8 w-8"
      onClick={copy}
    >
      {copied ? (
        <Check className="h-4 w-4 text-green-500" />
      ) : (
        <Copy className="h-4 w-4" />
      )}
    </Button>
  )
}


export const Route = createFileRoute(
  '/_authenticated/_dashboard/dashboard/units/$unitId',
)({
  component: RouteComponent,
  pendingComponent: () => (
    <div className="container mx-auto p-4">
      <div className="mb-6"><div className="h-10 w-32" /></div>
      <DetailsSkeleton />
    </div>
  ),
  loader: async ({ context, params: {unitId} }) => {
    const { user, organisationId, organisationName } = context;
    
    // Run all API calls in parallel instead of sequentially! 🚀
    const [unitData, unitVersionsData, unitStatusData] = await Promise.all([
      getUnitFn({data: {organisationId: organisationId || '', userId: user?.id || '', email: user?.email || '', unitId: unitId}}),
      getUnitVersionsFn({data: {organisationId: organisationId || '', userId: user?.id || '', email: user?.email || '', unitId: unitId}}),
      getUnitStatusFn({data: {organisationId: organisationId || '', userId: user?.id || '', email: user?.email || '', unitId: unitId}})
    ]);
    
    const publicServerConfig = context.publicServerConfig
    const publicHostname = publicServerConfig.PUBLIC_HOSTNAME || '<hostname>'


    return { 
      unitData: unitData, 
      unitStatus: unitStatusData,
      unitVersions: unitVersionsData.versions, 
      user, 
      organisationId,
      organisationName, 
      publicHostname,

    }
  }
})


function formatBytes(bytes: number) {
  if (bytes === 0) return '0 Bytes'
  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function formatDate(input: Date | string | number) {
  const date = input instanceof Date ? input : new Date(input)
  if (Number.isNaN(date.getTime())) return '—'
  return date.toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

function RouteComponent() {
  const data = Route.useLoaderData()
  const { unitData, unitVersions, unitStatus, organisationId, organisationName, publicHostname, user } = data
  const unit = unitData
  const router = useRouter()

  const handleUnlock = async () => {
    try {
      await unlockUnitFn({
        data: {
          userId: user?.id || '',
          organisationId: organisationId || '',
          email: user?.email || '',
          unitId: unit.id,
        },
      })
      toast({
        title: 'Unit unlocked',
        description: `Unit ${unit.name} was unlocked successfully.`,
        duration: 1000,
        variant: "default"
      })
      router.invalidate()
    } catch (error) {
      toast({
        title: 'Failed to unlock unit',
        description: `Failed to unlock unit ${unit.name}.`,
        duration: 5000,
        variant: "destructive"
      })
      console.error('Failed to unlock unit', error)
    }
  }

  const handleLock = async () => {
    try {
      await lockUnitFn({
        data: {
          userId: user?.id || '',
          organisationId: organisationId || '',
          email: user?.email || '',
          unitId: unit.id,
        },
      })
      toast({
        title: 'Unit locked',
        description: `Unit ${unit.name} was locked successfully.`,
        duration: 1000,
        variant: "default"
      })
      router.invalidate()
    } catch (error) {
      toast({
        title: 'Failed to lock unit',
        description: `Failed to lock unit ${unit.name}.`,
        duration: 5000,
        variant: "destructive"
      })
      console.error('Failed to lock unit', error)
    }
  }


  const handleDelete = async () => {
    try {
      await deleteUnitFn({
        data: {
          userId: user?.id || '',
          organisationId: organisationId || '',
          email: user?.email || '',
          unitId: unit.id,
        },
      })

      toast({
        title: 'Unit deleted',
        description: `Unit ${unit.name} was deleted successfully.`,
        duration: 1000,
        variant: "default"
      })
      router.invalidate()
    } catch (error) {
      console.error('Failed to delete unit', error)
      toast({
        title: 'Failed to delete unit',
        description: `Failed to delete unit ${unit.name}.`,
        duration: 5000,
        variant: "destructive"
      })
      return
    }
    setTimeout(() => router.navigate({ to: '/dashboard/units' }), 500)
  }

  const handleDownloadLatestState = async () => {
    try {
      const state : any = await downloadLatestStateFn({
        data: {
          userId: user?.id || '',
          organisationId: organisationId || '',
          email: user?.email || '',
          unitId: unit.id,
        },
      })
      downloadJson(state, `${unit.name}-latest-state.json`)
    } catch (error) {
      console.error('Failed to download latest state', error)
      toast({
        title: 'Failed to download latest state',
        description: `Failed to download latest state for unit ${unit.name}.`,
        duration: 5000,
        variant: "destructive"
      })
      return
    } 
  }
  
  const handleRestoreStateVersion = async (timestamp: string, lockId: string) => {
    try {
      await restoreUnitStateVersionFn({
        data: {
          userId: user?.id || '',
          organisationId: organisationId || '',
          email: user?.email || '',
          unitId: unit.id,
          timestamp: timestamp,
          lockId: lockId,
        },
      })
      toast({
        title: 'State version restored',
        description: `State version ${timestamp} was restored successfully.`,
        duration: 1000,
        variant: "default"
      })
      router.invalidate()
    } catch (error) {
      console.error('Failed to restore state version', error)
      toast({
        title: 'Failed to restore state version',
        description: `Failed to restore state version ${timestamp}.`,
        duration: 5000,
        variant: "destructive"
      })
      return
    }
  }

  return (
    <div className="container mx-auto p-4">
      <div className="mb-6 flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button variant="ghost" asChild>
            <Link to="/dashboard/units">
              <ArrowLeft className="mr-2 h-4 w-4" /> Back to Units
            </Link>
          </Button>
          <div className="flex gap-2">
              <Badge variant={unitStatus.status === "green" ? "secondary" : "destructive"}>
                {unitStatus.status === "green" ? (
                  <Check className="mr-2 h-3 w-3" />
                ) : (
                  <RefreshCcw className="mr-2 h-3 w-3" />
                )}
                {unitStatus.status === "green" ? "Up-to-date" : "Needs re-apply"}
              </Badge>
            <Badge variant={unit.locked ? "destructive" : "secondary"}>
              {unit.locked ? <Lock className="mr-2 h-3 w-3" /> : <Unlock className="mr-2 h-3 w-3" />}
              {unit.locked ? "Locked" : "Unlocked"}
            </Badge>
          </div>
        </div>
        
        <div className="flex items-center gap-2">
        <Button variant="outline" className="gap-2" onClick={handleDownloadLatestState}>
            <Download className="h-4 w-4"  />
            Download Latest State
          </Button>
          {unit.locked && <Button variant="outline" className="gap-2" onClick={handleUnlock}>
            <Unlock className="h-4 w-4" />
            Unlock
          </Button>}
          {!unit.locked && <Button variant="outline" className="gap-2" onClick={handleLock}>
            <Lock className="h-4 w-4" />
            Lock
          </Button>}

        </div>
      </div>

      <div className="grid gap-6">
        <Card>
          <CardHeader>
            <CardTitle className="text-2xl">{unit.name}</CardTitle>
            <CardDescription>
              ID: {unit.id}
            </CardDescription>
            <CardDescription>
              Last updated {formatDate(unit.updated)} • {formatBytes(unit.size)}
            </CardDescription>
          </CardHeader>
        </Card>

        <Tabs defaultValue="setup">
          <TabsList>
            <TabsTrigger value="setup">Setup</TabsTrigger>
            <TabsTrigger value="versions">State versions</TabsTrigger>
            <TabsTrigger value="settings">Settings</TabsTrigger>
          </TabsList>

          <TabsContent value="setup" className="mt-6">
            <UnitConfigureInstructions
              unitId={unit.id}
              organisationId={organisationId}
              publicHostname={publicHostname}
              showNextActions={false}
            />
            <div className="mt-4 rounded-md border bg-muted/30 p-4">
              <p className="text-sm text-muted-foreground mb-2">
                Want PR automation? Connect your VCS to trigger plans and applies from pull requests.
              </p>
              <Button asChild>
                <Link to="/dashboard/onboarding" search={{ step: 'github' } as any}>
                  Connect VCS for PR automation
                </Link>
              </Button>
            </div>
          </TabsContent>
          
          <TabsContent value="versions" className="mt-6">
            <Card>
              <CardHeader>
                <CardTitle>Version History</CardTitle>
                <CardDescription>Previous versions of this unit</CardDescription>
              </CardHeader>
              <CardContent>
                {(!unitVersions || unitVersions.length === 0) ? (
                  <div className="p-10 border border-dashed rounded-md text-center text-sm text-muted-foreground">
                    No versions yet. A version will appear after the first state is uploaded.
                  </div>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Hash</TableHead>
                        <TableHead className="w-[120px]">Size</TableHead>
                        <TableHead className="w-[230px]">Date</TableHead>
                        <TableHead className="w-[220px] text-right">Actions</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {unitVersions.map((version: any) => {
                        const shortHash = String(version.hash).slice(0, 8)
                        return (
                          <TableRow key={version.hash}>
                            <TableCell>
                              <code className="text-xs">{shortHash}</code>
                            </TableCell>
                            <TableCell>{formatBytes(Number(version.size) || 0)}</TableCell>
                            <TableCell>{formatDate(version.timestamp)}</TableCell>
                            <TableCell className="text-right">
                              <div className="flex items-center justify-end gap-2">
                                {!version.isLatest && (
                                  <Button variant="outline" size="sm" onClick={() => handleRestoreStateVersion(version.timestamp, version.lockId)}>
                                    <History className="mr-2 h-4 w-4" />
                                    Restore
                                  </Button>
                                )}
                              </div>
                            </TableCell>
                          </TableRow>
                        )
                      })}
                    </TableBody>
                  </Table>
                )}
              </CardContent>
            </Card>
          </TabsContent>



          <TabsContent value="settings" className="mt-6">
            <Card>
              <CardHeader>
                <CardTitle>Dangerous Operations</CardTitle>
                <CardDescription>These operations can potentially cause data loss. Use with caution.</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-8">
                  <div>
                    <h3 className="text-sm font-medium mb-2">Force Push State</h3>
                    <p className="text-sm text-muted-foreground mb-4">
                      This will overwrite the remote state with your local state, ignoring any locks or version history.
                      Only use this if you are absolutely sure your local state is correct.
                    </p>
                    <UnitStateForceUploadDialog userId={user?.id || ''} organisationId={organisationId || ''} userEmail={user?.email || ''} unitId={unit.id} isDisabled={unit.locked} />
                  </div>

                  <div className="pt-4 border-t">
                    <h3 className="text-sm font-medium mb-2">Delete Unit</h3>
                    <p className="text-sm text-muted-foreground mb-4">
                      This will permanently delete this unit and all of its version history. 
                      This action cannot be undone. Make sure to back up any important state before proceeding.
                    </p>
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button variant="destructive" className="gap-2">
                          <Trash2 className="h-4 w-4" />
                          Delete Unit
                        </Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>Delete this unit?</AlertDialogTitle>
                          <AlertDialogDescription>
                            This action cannot be undone. This will permanently delete the unit
                            and all of its version history.
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>Cancel</AlertDialogCancel>
                          <AlertDialogAction onClick={handleDelete}>Delete</AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  </div>
                </div>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </div>
    </div>
  )
}
