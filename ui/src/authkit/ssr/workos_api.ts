import { getWorkOS } from "./workos";
import { serverCache } from "@/lib/cache.server";

export async function createOrgForUser(userId: string, orgName: string) {
  try {
    const org = await getWorkOS().organizations.createOrganization({
      name: orgName,
    });
    
    await getWorkOS().userManagement.createOrganizationMembership({
      organizationId: org.id,
      userId: userId,
      roleSlug: 'admin'
    });

    return org;
  } catch (error) {
    console.error('Error creating organization:', error);
    throw error;
  }
}
  

export async function listUserOrganizationMemberships(userId: string) {
  try {
    // Check cache first
    const cachedMemberships = serverCache.getUserMemberships(userId);
    if (cachedMemberships) {
      console.log(`✅ User memberships cache hit for ${userId}`);
      return cachedMemberships;
    }
    
    // Cache miss - fetch from WorkOS
    console.log(`❌ User memberships cache miss, fetching from WorkOS for ${userId}`);
    const memberships = await getWorkOS().userManagement.listOrganizationMemberships({
      userId,
    });
    
    // Store full membership objects in cache
    serverCache.setUserMemberships(userId, memberships.data);
    
    return memberships.data;
  } catch (error) {
    console.error('Error fetching user memberships:', error);
    throw error;
  }
}

export async function getOrganisationDetails(orgId: string) {
  try {
    // Check cache first
    const cached = serverCache.getOrg(orgId);
    if (cached) {
      return cached;
    }
    
    // Cache miss - fetch from WorkOS
    const org = await getWorkOS().organizations.getOrganization(orgId);
    serverCache.setOrg(orgId, org);
    
    return org;
  } catch (error) {
    console.error('Error fetching organization details:', error);
    throw error;
  }
}

export async function getOranizationsForUser(userId: string) {
  try {
    // Check cache first
    const cachedMemberships = serverCache.getUserMemberships(userId);
    if (cachedMemberships) {
      console.log(`✅ Organizations cache hit for ${userId}`);
      return cachedMemberships;
    }
    
    // Cache miss - fetch from WorkOS
    console.log(`❌ Organizations cache miss, fetching from WorkOS for ${userId}`);
    const memberships = await getWorkOS().userManagement.listOrganizationMemberships({
      userId: userId,
    });
    
    // Store full membership objects in cache
    serverCache.setUserMemberships(userId, memberships.data);
    
    return memberships.data;
  } catch (error) {
    console.error('Error fetching user organizations:', error);
    throw error;
  }
}

export async function listUserOrganizationInvitations(email: string) {
  try {
    const invitations = await getWorkOS().userManagement.listInvitations({
      email: email,
    });
    
    return invitations.data;
  } catch (error) {
    console.error('Error fetching user invitations:', error);
    throw error;
  }
}

export async function getUserOrganization(userId: string) {
  try {
    const memberships = await listUserOrganizationMemberships(userId);
    if (memberships.length === 0) {
      throw new Error('User is not a member of any organization');
    }
    return memberships[0];
  } catch (error) {
    console.error('Error fetching user organization:', error);
    throw error;
  }
}
  