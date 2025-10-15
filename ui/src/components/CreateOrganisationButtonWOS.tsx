import { createOrganization } from "@/authkit/serverFunctions";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useState } from "react";
import { useToast } from "@/hooks/use-toast";


export default function CreateOrganizationBtn({ userId }: { userId: string }) {
  const [name, setName] = useState("");
  const [open, setOpen] = useState(false);
  const { toast } = useToast();
  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    
      try {
        const organization = await createOrganization({ data: { name: name, userId: userId } });
        toast({
          title: "Organization created",
          description: "The page will now reload to refresh organisations list. To use this new organization, select it from the list.",
          duration: 5000,
          variant: "default"
        });
        setOpen(false);
        window.setTimeout(() => {
          window.location.reload();
        }, 5000);
      } catch (error) {
        console.error("Failed to create organization:", error);
      }
    
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button onClick={() => setOpen(true)} variant="outline" className="w-full">Create New Organization</Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create New Organization</DialogTitle>
          <DialogDescription>
            Create a new organization to collaborate with your team.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <Label htmlFor="name">Organization Name</Label>
            <Input
              id="name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Enter organization name"
              required
            />
          </div>
          <Button type="submit">Create</Button>
        </form>
      </DialogContent>
    </Dialog>
  );
}
