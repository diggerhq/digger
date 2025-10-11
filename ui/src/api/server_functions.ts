import { createServerFn } from "@tanstack/react-start";
import { Job, OrgSettings, Project, Repo } from './types'
import { fetchRepos, testSlackWebhook } from "./backend";
import { fetchProject, fetchProjects, getOrgSettings, updateOrgSettings, updateProject } from "./api";

export const getOrgSettingsFn = createServerFn({method: 'GET'})
  .inputValidator((data : {userId: string, organisationId: string}) => data)
  .handler(async ({ data }) => {
    const settings : any = await getOrgSettings(data.organisationId, data.userId)
    return settings
})

export const updateOrgSettingsFn = createServerFn({method: 'POST'})
  .inputValidator((data : {userId: string, organisationId: string, settings: OrgSettings}) => data)
  .handler(async ({ data }) => {
    const settings : any = await updateOrgSettings(data.organisationId, data.userId, data.settings)
    return settings.result
})

export const getProjectsFn = createServerFn({method: 'GET'})
    .inputValidator((data : {userId: string, organisationId: string}) => {
      if (!data.userId || !data.organisationId) {
        throw new Error('Missing required fields: userId and organisationId are required')
      }
      return data
    })
    .handler(async ({ data }) => {
      try {
        const projects : any = await fetchProjects(data.organisationId, data.userId)
        return projects.result || []
      } catch (error) {
        console.error('Error in getProjectsFn:', error)
        return []
      }
    })


export const updateProjectFn = createServerFn({method: 'POST'})
    .inputValidator((data : {projectId: string, driftEnabled: boolean, organisationId: string, userId: string}) => data)
    .handler(async ({ data }) => {
    const project : any = await updateProject(data.projectId, data.driftEnabled, data.organisationId, data.userId)
    return project.result
  })

export const getReposFn = createServerFn({method: 'GET'})
    .inputValidator((data : {organisationId: string, userId: string}) => data)
    .handler(async ({ data }) => {
    let repos = []
    try {
        const reposData :any = await fetchRepos(data.organisationId, data.userId)
        repos = reposData.result
      } catch (error) {
        console.error('Error fetching repos:', error)
        throw error
      }
      return repos
  })

export const getProjectFn = createServerFn({method: 'GET'})
    .inputValidator((data : {projectId: string, organisationId: string, userId: string}) => data)
    .handler(async ({ data }) => {
    const project : any = await fetchProject(data.projectId, data.organisationId, data.userId)
    return project
  })


export const getRepoDetailsFn = createServerFn({method: 'GET'})
    .inputValidator((data : {repoId: string, organisationId: string, userId: string}) => data)
    .handler(async ({ data }) => {
      const { repoId, organisationId, userId } = data;
      let allJobs: Job[] = [];
      let repo: Repo 
      try {
        const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/api/repos/${repoId}/jobs`, {
          method: 'GET',
          headers: {
            'Authorization': `Bearer ${process.env.ORCHESTRATOR_BACKEND_SECRET}`,
            'DIGGER_ORG_ID': organisationId,
            'DIGGER_USER_ID': userId,
            'DIGGER_ORG_SOURCE': 'workos',
          },
        });
      
        if (!response.ok) {
          throw new Error('Failed to fetch jobs');
        }
      
        const data :any = await response.json();
        repo = data.repo    
        allJobs = data.jobs || []
    
      } catch (error) {
        console.error('Error fetching jobs:', error);
        allJobs = [];
        throw error
      }
  
      return { repo, allJobs }
    })



export const testSlackWebhookFn = createServerFn({method: 'POST'})
    .inputValidator((data : {notification_url: string}) => data)
    .handler(async ({ data }) => {
    const response : any = await testSlackWebhook(data.notification_url)
    return response
  })
  