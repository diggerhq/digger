import { createServerFn } from "@tanstack/react-start";
import { Job, Repo } from './types'
import { fetchRepos } from "./backend";
import { fetchProjects, updateProject } from "./api";



export const getProjectsFn = createServerFn({method: 'GET'})
    .inputValidator((data : {userId: string, organisationId: string}) => data)
    .handler(async ({ data }) => {
    const projects : any = await fetchProjects(data.organisationId, data.userId)
    return projects.result
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


export const getRepoDetailsFn = createServerFn({method: 'GET'})
    .inputValidator((data : {repoId: string, organisationId: string, userId: string}) => data)
    .handler(async ({ data }) => {
      const { repoId, organisationId, userId } = data;
      let allJobs: Job[] = [];
      let repo: Repo 
      try {
        const response = await fetch(`${process.env.BACKEND_URL}/api/repos/${repoId}/jobs`, {
          method: 'GET',
          headers: {
            'Authorization': `Bearer ${process.env.BACKEND_SECRET}`,
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
