import { createOrganization } from "@/authkit/serverFunctions";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useState } from "react";
import { useToast } from "@/hooks/use-toast";

// Organization name validation to match backend rules
const ORG_NAME_PATTERN = /^[a-zA-Z0-9][a-zA-Z0-9_\- :.]*$/;
const MIN_LENGTH = 3;
const MAX_LENGTH = 255;

function validateOrgName(name: string): string | null {
  const trimmed = name.trim();
  
  if (trimmed.length < MIN_LENGTH) {
    return `Organization name must be at least ${MIN_LENGTH} characters`;
  }
  
  if (trimmed.length > MAX_LENGTH) {
    return `Organization name must be at most ${MAX_LENGTH} characters`;
  }
  
  if (!ORG_NAME_PATTERN.test(trimmed)) {
    return "Organization name must start with a letter or number, and can only contain letters, numbers, hyphens, underscores, spaces, colons, and periods";
  }
  
  return null;
}

export default function CreateOrganizationBtn({ userId, email }: { userId: string, email: string }) {
  const [name, setName] = useState("");
  const [validationError, setValidationError] = useState<string | null>(null);
  const [open, setOpen] = useState(false);
  const { toast } = useToast();

  const handleNameChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value;
    setName(value);
    
    // Clear validation error when user starts typing again
    if (validationError) {
      setValidationError(null);
    }
  };

  const handleOpenChange = (isOpen: boolean) => {
    setOpen(isOpen);
    // Reset form when dialog is closed
    if (!isOpen) {
      setName("");
      setValidationError(null);
    }
  };

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    
    // Validate before submitting
    const error = validateOrgName(name);
    if (error) {
      setValidationError(error);
      return;
    }
    
    try {
      const organization = await createOrganization({ data: { name: name.trim(), userId: userId, email: email } });
      toast({
        title: "Organization created",
        description: "The page will now reload to refresh organisations list. To use this new organization, select it from the list.",
        duration: 5000,
        variant: "default"
      });
      handleOpenChange(false); // Close dialog and reset form
      window.setTimeout(() => {
        window.location.reload();
      }, 5000);
    } catch (error) {
      console.error("Failed to create organization:", error);
      toast({
        title: "Failed to create organization",
        description: error instanceof Error ? error.message : "An error occurred while creating the organization",
        duration: 5000,
        variant: "destructive"
      });
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button variant="outline" className="w-full">Create New Organization</Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create New Organization</DialogTitle>
          <DialogDescription>
            Create a new organization to collaborate with your team.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="name">Organization Name</Label>
            <Input
              id="name"
              value={name}
              onChange={handleNameChange}
              placeholder="Enter organization name"
              className={validationError ? "border-destructive" : ""}
              required
            />
            {validationError && (
              <p className="text-sm text-destructive">{validationError}</p>
            )}
            <p className="text-xs text-muted-foreground">
              Must be 3-255 characters, start with a letter or number
            </p>
          </div>
          <Button type="submit">Create</Button>
        </form>
      </DialogContent>
    </Dialog>
  );
}
