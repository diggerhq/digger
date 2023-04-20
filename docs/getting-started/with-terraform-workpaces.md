# With Terraform Workpaces

In this tutorial we will be using a repository in order to configure a terraform pipeline [https://github.com/diggerhq/digger\_demo\_workspaces](https://github.com/diggerhq/digger\_demo\_workspaces). In order to use Terraform workspaces with Digger we follow the steps below:

Let's create our first pull request with a change and see this in action:

1. Fork the [demo repository](https://github.com/diggerhq/digger\_demo\_workspaces)
2. Enable Actions (by default workflows won't trigger in a fork)

<figure><img src="../.gitbook/assets/image (3).png" alt=""><figcaption></figcaption></figure>

3. In your repository settings > Actions ensure that the Workflow Read and Write permissions are assigned - This will allow the workflow to post comments on your PRs

<figure><img src="../.gitbook/assets/image (1).png" alt=""><figcaption></figcaption></figure>

4.  Add environment variables into your Github Action Secrets (cloud keys are a requirement since digger needs to connect to your account for coordinating locks).  In this case we are setting AWS keys for this demo but you can follow [GCP](https://diggerhq.gitbook.io/digger-docs/cloud-providers/gcp) or [Azure](https://diggerhq.gitbook.io/digger-docs/cloud-providers/azure) steps as seen fit.



    <figure><img src="../.gitbook/assets/image (2).png" alt=""><figcaption></figcaption></figure>
5. make a change in `main.tf` and create a PR -you can just rename an existing null resource to test it. Notice that we have digger.yml where we have two workspaces defined: one for dev and the other for prod. Therefore Digger will create two plans for us:

<figure><img src="../.gitbook/assets/Screen Shot 2023-03-30 at 5.22.39 PM.png" alt=""><figcaption></figcaption></figure>



8. Lets apply the PR and merge it to unlock the flow for our colleagues. We can do this by commenting `digger apply`

<figure><img src="../.gitbook/assets/Screen Shot 2023-03-30 at 4.32.40 PM.png" alt=""><figcaption></figcaption></figure>

**Conclusion**

In this tutorial we created a workspace enabled example with terraform and workspaces to provision two environments with independent states.
