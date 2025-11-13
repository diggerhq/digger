import { Button, Flex } from '@radix-ui/themes';
import { Link } from '@tanstack/react-router';
import type { User } from '@workos-inc/node';

export default function SignInButton({ large, user, url }: { large?: boolean; user: User | null; url: string }) {
  if (user) {
    return (
      <Flex gap="3">
        <Button asChild size={large ? '3' : '2'}>
          <Link to="/logout">Sign Out</Link>
        </Button>
      </Flex>
    );
  }

  return (
    <Button asChild size={large ? '3' : '2'} className="cursor-pointer">
      <a
        href={url}
        style={{
          color: 'white',
          background:
            'linear-gradient(90deg, #6D28D9 0%, #3B82F6 100%)',
          padding: '0.65em 2em',
          borderRadius: '8px',
          fontWeight: 'bold',
          letterSpacing: '0.03em',
          fontSize: '1.1em',
          boxShadow: '0 2px 16px 0 rgba(59, 130, 246, 0.35)',
          border: 'none',
          textShadow: '0 2px 8px rgba(59,130,246,0.17)'
        }}
      >
        Sign In To Get Started
      </a>
    </Button>
  );
}
