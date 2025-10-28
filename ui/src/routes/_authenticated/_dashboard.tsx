import { redirect, createFileRoute, Outlet, useLocation } from '@tanstack/react-router';
import { getSignInUrl } from '../../authkit/serverFunctions';
import { SidebarProvider, Sidebar, SidebarHeader, SidebarContent, SidebarGroup, SidebarGroupLabel, SidebarGroupContent, SidebarMenu, SidebarMenuItem, SidebarMenuButton, SidebarTrigger } from '@/components/ui/sidebar';
import { Link } from '@tanstack/react-router';
import { GitBranch, Folders, Waves, Settings, CreditCard, LogOut, Cuboid} from 'lucide-react';

export const Route = createFileRoute('/_authenticated/_dashboard')({
    component: DashboardComponent,
    loader: async ({ context }) => {
        const { user, organisationName } = context;
        return { user, organisationName };
    },
});

function DashboardComponent() {
    const { user, organisationName } = Route.useLoaderData();
    const location = useLocation(); 
    return (
        <SidebarProvider>
        <div className="flex h-screen w-full">
          <Sidebar>
            <SidebarHeader className="text-center">
              <h2 className="text-xl font-bold mb-2">🌮 OpenTACO</h2>
              <div className="px-4">
                <div className="h-[1px] bg-border mb-2" />
                <h3>
                  <Link 
                    to="/dashboard/settings" 
                    className="text-sm text-muted-foreground hover:text-primary transition-colors duration-200"
                  >
                    {organisationName}
                  </Link>
                </h3>
                <div className="h-[1px] bg-border mt-2" />
              </div>
            </SidebarHeader>
            <SidebarContent>
              <SidebarGroup>
                <SidebarGroupLabel>Menu</SidebarGroupLabel>
                <SidebarGroupContent>
                  <SidebarMenu>

                  <SidebarMenuItem>
                      <SidebarMenuButton asChild isActive={location.pathname.startsWith('/dashboard/units')}>
                        <Link to="/dashboard/units">
                          <Cuboid className="mr-2 h-4 w-4" />
                          <span>Units</span>
                        </Link>
                      </SidebarMenuButton>
                    </SidebarMenuItem>

                    <SidebarMenuItem>
                      <SidebarMenuButton asChild isActive={location.pathname.startsWith('/dashboard/repos')}>
                        <Link to="/dashboard/repos">
                          <GitBranch className="mr-2 h-4 w-4" />
                          <span>Repos</span>
                        </Link>
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                    
                    <SidebarMenuItem>
                      <SidebarMenuButton asChild isActive={location.pathname.startsWith('/dashboard/projects')}>
                        <Link to="/dashboard/projects">
                          <Folders className="mr-2 h-4 w-4" />
                          <span>Projects</span>
                        </Link>
                      </SidebarMenuButton>
                    </SidebarMenuItem>


                    <SidebarMenuItem>
                      <SidebarMenuButton asChild isActive={location.pathname.startsWith('/dashboard/drift')}>
                        <Link to="/dashboard/drift">
                          <Waves className="mr-2 h-4 w-4" />
                          <span>Drift</span>
                        </Link>
                      </SidebarMenuButton>
                    </SidebarMenuItem> 

                    <SidebarMenuItem>
                      <SidebarMenuButton asChild isActive={location.pathname.startsWith('/dashboard/settings')}>
                        <Link to="/dashboard/settings/tokens">
                          <Settings className="mr-2 h-4 w-4" />
                          <span>Settings</span>
                        </Link>
                      </SidebarMenuButton>
                    </SidebarMenuItem>



                  </SidebarMenu>
                </SidebarGroupContent>
              </SidebarGroup>
            </SidebarContent>
            <div className="mt-auto p-4">
              <Link to="/logout" className="flex items-center">
                <LogOut className="mr-2 h-4 w-4" />
                <span>Logout</span>
              </Link>
            </div>
          </Sidebar>
          <main className="flex-1 overflow-y-auto">
            <div className="p-4">
              <SidebarTrigger />
              <Outlet />
            </div>
          </main>
        </div>
      </SidebarProvider>    
    )
};
