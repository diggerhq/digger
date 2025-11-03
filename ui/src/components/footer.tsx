import { Card, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';

export default function Footer() {
  return (
    <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 sm:gap-5">
      <a href="https://workos.com/docs" rel="noreferrer" target="_blank" className="no-underline">
        <Card className="hover:bg-accent transition-colors h-full">
          <CardHeader>
            <CardTitle className="text-lg mb-1">Documentation</CardTitle>
            <CardDescription>View integration guides and SDK documentation.</CardDescription>
          </CardHeader>
        </Card>
      </a>
      <a href="https://workos.com/docs/reference" rel="noreferrer" target="_blank" className="no-underline">
        <Card className="hover:bg-accent transition-colors h-full">
          <CardHeader>
            <CardTitle className="text-lg mb-1">API Reference</CardTitle>
            <CardDescription>Every WorkOS API method and endpoint documented.</CardDescription>
          </CardHeader>
        </Card>
      </a>
      <a href="https://workos.com" rel="noreferrer" target="_blank" className="no-underline">
        <Card className="hover:bg-accent transition-colors h-full">
          <CardHeader>
            <CardTitle className="text-lg mb-1">WorkOS</CardTitle>
            <CardDescription>Learn more about other WorkOS products.</CardDescription>
          </CardHeader>
        </Card>
      </a>
    </div>
  );
}
