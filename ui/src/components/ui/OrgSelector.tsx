// Commented out - legacy file, not currently used
// import { getApiProps } from "@repo/env";
// import { UsersManagement, WorkOsWidgets } from "@workos-inc/widgets";
// import { OrganizationSwitcher } from "@workos-inc/widgets/organization-switcher";
// import { WorkOS } from '@workos-inc/node';
// import { switchToOrganization } from "@/server-functions/switch-to-organization";

// export function getApiProps() {
//     if (typeof process === "undefined") {
//       throw new Error("getApiProps must be called in a Node.js environment");
//     }
  
//     const props: {
//       apiHostname?: string;
//       https?: boolean;
//       port?: number;
//     } = {};
  
//     if (process.env.WORKOS_API_HOSTNAME) {
//       props.apiHostname = process.env.WORKOS_API_HOSTNAME;
//     }
//     if (process.env.WORKOS_API_HTTPS) {
//       props.https = process.env.WORKOS_API_HTTPS === "true";
//     }
//     if (process.env.WORKOS_API_PORT) {
//       props.port = Number.parseInt(process.env.WORKOS_API_PORT, 10);
//     }
//     return props;
//   }

// const workos = new WorkOS(process.env.WORKOS_API_KEY!);

export default async function OrgSelector(props: { user: any, organizationId: string | undefined }) {
  const { user, organizationId }: { user: any, organizationId: string | undefined } = props;
  if (!organizationId) {
    return <p>User does not belong to an organization</p>;
  }
  // const authToken = await workos.widgets.getToken({
  //   userId: user.id,
  //   organizationId,
  //   scopes: ["widgets:users-table:manage"],
  // });
  return (
    <div className="flex gap-2 p-2 flex-col items-start w-full">
    {/* <WorkOsWidgets>
        <OrganizationSwitcher
          authToken={authToken}
          organizationLabel="My Teams"
          switchToOrganization={async ({ organizationId }) => {
            "use server";

            await switchToOrganization({
              organizationId,
              pathname: "/dashboard",
            });
          }}
        >
        </OrganizationSwitcher>
    </WorkOsWidgets> */}
    </div>
  );
}