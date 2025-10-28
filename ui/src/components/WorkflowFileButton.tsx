'use client'

import { Button } from "@/components/ui/button"
import { CheckCircle2 } from "lucide-react"

interface WorkflowFileButtonProps {
  onClick: () => void
}

export function WorkflowFileButton({ onClick }: WorkflowFileButtonProps) {

  const handleClick = () => {
    // trackWorkflowFileAdded(user, organizationId)
    onClick()
  }

  return (
    <Button onClick={handleClick}>
      <CheckCircle2 className="mr-2 h-4 w-4" /> I've added the workflow file
    </Button>
  )
}