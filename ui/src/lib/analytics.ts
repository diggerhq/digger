import posthog from 'posthog-js'

// Analytics event tracking utility functions
export const trackEvent = (eventName: string, properties?: Record<string, any>) => {
  if (typeof window !== 'undefined' && posthog) {
    let res = posthog.capture(eventName, properties)
    console.log("trackEventResult", res)
  }
}

// Analytics event tracking with user identification
export const trackEventWithUser = (eventName: string, user: any, organizationId?: string, properties?: Record<string, any>) => {
  if (typeof window !== 'undefined' && posthog) {
    // Identify user if we have the data
    if (user?.id) {
      posthog.identify(user.id, {
        email: user.email,
        organization_id: organizationId
      })
    }
    
    let res = posthog.capture(eventName, properties)
    console.log("trackEventWithUserResult", res)
  }
}

// Repository connection events
export const trackConnectMoreRepositories = (user?: any, organizationId?: string) => {
  trackEventWithUser('connect_more_repositories_clicked', user, organizationId, {
    source: 'repos_page',
    action: 'navigation_to_onboarding'
  })
}

export const trackConnectWithGithub = (source: 'onboarding' | 'add_connection_dialog', user?: any, organizationId?: string) => {
  trackEventWithUser('connect_with_github_clicked', user, organizationId, {
    source,
    action: 'redirect_to_github_app'
  })
}

export const trackGithubAppInstallation = (status: 'success' | 'failure', installationId?: string, error?: string, user?: any, organizationId?: string) => {
  trackEventWithUser('github_app_installation', user, organizationId, {
    status,
    installation_id: installationId,
    error: error || null
  })
}

export const trackGithubAppInstallationComplete = (status: 'success' | 'failure', installationId?: string, error?: string, user?: any, organizationId?: string) => {
  trackEventWithUser('github_app_installation_complete', user, organizationId, {
    status,
    installation_id: installationId,
    error: error || null
  })
}

// Onboarding step events
export const trackWorkflowFileAdded = (user?: any, organizationId?: string) => {
  trackEventWithUser('workflow_file_added', user, organizationId, {
    step: 'workflow',
    action: 'onboarding_step_completed'
  })
}

export const trackDiggerYmlAdded = (user?: any, organizationId?: string) => {
  trackEventWithUser('digger_yml_added', user, organizationId, {
    step: 'digger_config',
    action: 'onboarding_step_completed'
  })
}

export const trackPRCreated = (user?: any, organizationId?: string) => {
  trackEventWithUser('pr_created', user, organizationId, {
    step: 'terraform_pr',
    action: 'onboarding_completed'
  })
}

export const trackProjectDriftToggled = (user?: any, organizationId?: string, projectId?: string, status?: string) => {
  trackEventWithUser('monitored_project_toggled', user, organizationId, {
    project_id: projectId,
    status: status
  })
}


export const trackBillingModalShownToUser = (user?: any, organizationId?: string) => {
  trackEventWithUser('billing_modal_shown_to_user', user, organizationId, {
    action: 'billing_modal_shown'
  })
}

export const trackBillingModalClosed = (user?: any, organizationId?: string) => {
  trackEventWithUser('billing_modal_closed', user, organizationId, {
    action: 'billing_modal_closed'
  })
}

export const trackBillingModalAccepted = (user?: any, organizationId?: string) => {
  trackEventWithUser('billing_modal_accepted', user, organizationId, {
    action: 'billing_modal_accepted'
  })
}