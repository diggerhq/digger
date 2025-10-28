import React, { useState } from 'react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Upload } from 'lucide-react'
import { forcePushStateFn } from '@/api/statesman_serverFunctions'
import { toast } from '@/hooks/use-toast'

export default function UnitStateForceUploadDialog({ userId, organisationId, userEmail, unitId, isDisabled }: { userId: string, organisationId: string, userEmail: string, unitId: string, isDisabled: boolean }) {
  const [open, setOpen] = useState(false)
  const [fileContent, setFileContent] = useState<string | null>(null)
  const [status, setStatus] = useState<'idle' | 'loading' | 'success' | 'error'>('idle')

  const handleOpenChange = (nextOpen: boolean) => {
    setOpen(nextOpen)
    if (!nextOpen) {
      setFileContent(null)
      setStatus('idle')
    }
  }

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    const text = await file.text()
    setFileContent(text)
  }

  const handleUpload = async () => {
    if (!fileContent) {
      alert('Please select a file first.')
      return
    }
    try {
      setStatus('loading')
      await forcePushStateFn({
        data: {
          userId: userId,
          organisationId: organisationId,
          email: userEmail,
          unitId: unitId,
          state: fileContent,
        },
      })
      toast({
        title: 'State uploaded',
        description: `State for unit ${unitId} was uploaded successfully.`,
        duration: 4000,
        variant: 'default',
      })
      setStatus('success')
      setOpen(false)
    } catch (err) {
      console.error(err)
      setStatus('error')
      toast({
        title: 'Upload failed',
        description: 'Failed to upload state. Please try again.',
        duration: 5000,
        variant: 'destructive',
      })
    }
  }
  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button variant="destructive" className="gap-2" disabled={isDisabled}>
          <Upload className="h-4 w-4" />
          Force Push State
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Force push state</DialogTitle>
          <DialogDescription>
            This will overwrite the remote state with your selected file. Only use this if you are absolutely sure your local state is correct.
          </DialogDescription>
        </DialogHeader>
        <div className="p-2 space-y-4">
          <input type="file" onChange={handleFileChange} />
          <div>
            <Button
              onClick={handleUpload}
              disabled={status === 'loading'}
              className="gap-2"
              variant="destructive"
            >
              {status === 'loading' ? 'Uploading…' : 'Upload new state file'}
            </Button>
          </div>
          {status === 'success' && (
            <p className="text-green-600">Upload successful!</p>
          )}
          {status === 'error' && (
            <p className="text-red-600">Upload failed.</p>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}


