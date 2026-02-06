import { createServerFn } from "@tanstack/react-start";
import { Job, OrgSettings, Project, Repo } from './orchestrator_types'
import { fetchRepos } from "./orchestrator_repos";
import { getOrgSettings, updateOrgSettings } from "./orchestrator_orgs";
import { testSlackWebhook } from "./drift_slack";
import { fetchProjects, updateProject, fetchProject } from "./orchestrator_projects";
import { requireAuth } from "./helpers";

export const getOrgSettingsFn = createServerFn({method: 'GET'})
  .handler(async () => {
    const auth = await requireAuth();
    const settings : any = await getOrgSettings(auth.organizationId, auth.userId)
    return settings
})

export const updateOrgSettingsFn = createServerFn({method: 'POST'})
  .inputValidator((data : {settings: OrgSettings}) => data)
  .handler(async ({ data }) => {
    const auth = await requireAuth();
    const settings : any = await updateOrgSettings(auth.organizationId, auth.userId, data.settings)
    return settings.result
})

export const getProjectsFn = createServerFn({method: 'GET'})
    .handler(async () => {
      const auth = await requireAuth();
      try {
        const projects : any = await fetchProjects(auth.organizationId, auth.userId)
        return projects.result || []
      } catch (error) {
        console.error('Error in getProjectsFn:', error)
        return []
      }
    })


export const updateProjectFn = createServerFn({method: 'POST'})
    .inputValidator((data : {projectId: string, driftEnabled: boolean}) => data)
    .handler(async ({ data }) => {
      const auth = await requireAuth();
      const project : any = await updateProject(data.projectId, data.driftEnabled, auth.organizationId, auth.userId)
      return project.result
  })

export const getReposFn = createServerFn({method: 'GET'})
    .handler(async () => {
      const auth = await requireAuth();
      let repos = []
      try {
          const reposData :any = await fetchRepos(auth.organizationId, auth.userId)
          repos = reposData.result
        } catch (error) {
          console.error('Error fetching repos:', error)
          throw error
        }
        return repos
  })

export const getProjectFn = createServerFn({method: 'GET'})
    .inputValidator((data : {projectId: string}) => data)
    .handler(async ({ data }) => {
      const auth = await requireAuth();
      const project : any = await fetchProject(data.projectId, auth.organizationId, auth.userId)
      return project
  })


export const getRepoDetailsFn = createServerFn({method: 'GET'})
    .inputValidator((data : {repoId: string}) => data)
    .handler(async ({ data }) => {
      const auth = await requireAuth();
      const { repoId } = data;
      let allJobs: Job[] = [];
      let repo: Repo
      try {
        const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/api/repos/${repoId}/jobs`, {
          method: 'GET',
          headers: {
            'Authorization': `Bearer ${process.env.ORCHESTRATOR_BACKEND_SECRET}`,
            'DIGGER_ORG_ID': auth.organizationId,
            'DIGGER_USER_ID': auth.userId,
            'DIGGER_ORG_SOURCE': 'workos',
          },
        });

        if (!response.ok) {
          throw new Error('Failed to fetch jobs');
        }

        const result = await response.json();

        repo = result.repo
        allJobs = result.jobs || []

      } catch (error) {
        console.error('Error fetching jobs:', error);
        allJobs = [];
        throw error
      }

      return { repo, allJobs }
    })

export const switchToOrganizationFn = createServerFn({method: 'POST'})
    .inputValidator((data : { organisationId: string, pathname: string}) => data)
    .handler(async ({ data }) => {
      return null
    })
