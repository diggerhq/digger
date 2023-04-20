# With Terragrunt

In this tutorial we will be using a repository in order to configure a terraform pipeline [https://github.com/diggerhq/digger\_demo\_terragrunt](https://github.com/diggerhq/digger\_demo\_terragrunt). In order to use Terraform workspaces with Digger we follow the steps below:

Let's create our first pull request with a change and see this in action:

1. Fork the [demo repository](https://github.com/diggerhq/digger\_demo\_terragrunt)
2. Enable Actions (by default workflows won't trigger in a fork)

<figure><img src="../.gitbook/assets/image (3).png" alt=""><figcaption></figcaption></figure>

3. In your repository settings > Actions ensure that the Workflow Read and Write permissions are assigned - This will allow the workflow to post comments on your PRs

<figure><img src="../.gitbook/assets/image (1).png" alt=""><figcaption></figcaption></figure>

4.  Add environment variables into your Github Action Secrets (cloud keys are a requirement since digger needs to connect to your account for coordinating locks).  In this case we are setting AWS keys for this demo but you can follow [GCP](https://diggerhq.gitbook.io/digger-docs/cloud-providers/gcp) or [Azure](https://diggerhq.gitbook.io/digger-docs/cloud-providers/azure) steps as seen fit.



    <figure><img src="../.gitbook/assets/image (2).png" alt=""><figcaption></figcaption></figure>

{% hint style="info" %}
Note: There are several things here which are different from the other repositories here. Ofcourse we have a `terragrunt.hcl` file which marks that it is the root of our repository. We have set `terragrunt: true` in our digger.yml configuration. We also modified our action to install terragrunt prior to running the digger action.&#x20;
{% endhint %}

5. make a change in `main.tf` and create a PR - You should see the appropriate plan in the pull request as a comment.

<figure><img src="../.gitbook/assets/Screen Shot 2023-03-31 at 5.34.03 PM.png" alt=""><figcaption></figcaption></figure>

5. Lets apply the PR and merge it to unlock the flow for our colleagues. We can do this by commenting `digger apply`

<figure><img src="../.gitbook/assets/Screen Shot 2023-03-31 at 5.34.37 PM.png" alt=""><figcaption></figcaption></figure>

**Conclusion**

In this tutorial we created a terragrunt based project and used digger to plan and apply the resources in it within a collaorative environment.
