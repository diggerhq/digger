# Getting Started

If you prefer to watch a video instead you can watch this demo tutorial:

{% embed url="https://www.loom.com/embed/e201e639a73941e0b5508710377a6106" %}
Demo tutorial
{% endembed %}



In this tutorial we will be using a repository in order to configure a terraform pipeline [https://github.com/diggerhq/digger\_demo\_multienv](https://github.com/diggerhq/digger\_demo\_multienv). In this example we have terraform in two directories:

```
dev/
    main.tf
prod/
    main.tf
```

We want to set up a comment-based flow where the user creates a pull request with a change and sees the plan for the change. The user is required to apply the plan and it should succeed before the PR is merged to the main branch. This is achieved by commenting `digger plan` and `digger apply` on the pull request itself. This is only one option which digger supports.

Let's create our first pull request with a change and see this in action:

1. Fork the [demo repository](https://github.com/diggerhq/digger\_demo\_multienv)
2. Enable Actions (by default workflows won't trigger in a fork)

<figure><img src="../.gitbook/assets/image (3).png" alt=""><figcaption></figcaption></figure>

3. In your repository settings > Actions ensure that the Workflow Read and Write permissions are assigned - This will allow the workflow to post comments on your PRs

<figure><img src="../.gitbook/assets/image (1).png" alt=""><figcaption></figcaption></figure>

4. Add environment variables into your Github Action Secrets (cloud keys are a requirement since digger needs to connect to your account for coordinating locks)
   * AWS\_ACCESS\_KEY\_ID
   * AWS\_SECRET\_ACCESS\_KEY

<figure><img src="../.gitbook/assets/Screen Shot 2023-03-22 at 3.50.25 PM.png" alt=""><figcaption></figcaption></figure>

5. make a change in `dev/main.tf` and create a PR - this will create a lock

<figure><img src="../.gitbook/assets/Screen Shot 2023-03-22 at 3.51.25 PM.png" alt=""><figcaption></figcaption></figure>

6. comment `digger plan` - terraform plan output will be added as comment. If you don't see a comment (bug) - check out job output

<figure><img src="../.gitbook/assets/Screen Shot 2023-03-22 at 3.52.53 PM.png" alt=""><figcaption></figcaption></figure>

6. create another PR - plan or apply won’t work in this PR until the first lock is released
7. You should see `Locked by PR #1` comment. The action logs will display "Project locked" error message.

<figure><img src="../.gitbook/assets/Screen Shot 2023-03-22 at 3.57.15 PM.png" alt=""><figcaption></figcaption></figure>

8. Lets apply the first PR and merge it to unlock the flow for our colleagues

<figure><img src="../.gitbook/assets/Screen Shot 2023-03-22 at 4.01.17 PM.png" alt=""><figcaption></figcaption></figure>



**Conclusion**

In this tutorial we reused an existing sample repository to set up our first collaborative terraform environment which allows us to collaborate with our team and run terraform changes safely without conflicts.



**Next steps**

* Managing your own state backend
* Using digger with [GCP](../cloud-providers/gcp.md) and [Azure](../cloud-providers/azure.md)
* Using digger with [Bitbucket](../ci-systems/bitbucket.md), [Gitlab](../ci-systems/gitlab.md) and [Jenkins](../ci-systems/jenkins.md)

