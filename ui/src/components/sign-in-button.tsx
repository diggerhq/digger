import { Button } from '@/components/ui/button';
import { Link } from '@tanstack/react-router';
import type { User } from '@workos-inc/node';

export default function SignInButton({ large, user, url }: { large?: boolean; user: User | null; url: string }) {
  if (user) {
    return (
      <div className="flex gap-3">
        <Button asChild size={large ? 'lg' : 'default'}>
          <Link to="/logout">Sign Out</Link>
        </Button>
      </div>
    );
  }

  return (
    <Button asChild size={large ? 'lg' : 'default'} className="cursor-pointer">
      <a href={url}>Sign In To Get Started</a>
    </Button>
  );
}
