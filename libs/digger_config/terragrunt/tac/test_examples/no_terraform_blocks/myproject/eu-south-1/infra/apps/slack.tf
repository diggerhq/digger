module "main_slack" {
  source = "github.com/terraform-aws-modules/terraform-aws-notify-slack"

  sns_topic_name = "main-slack-topic"

  slack_webhook_url = "https://hooks.slack.com/services/AAA/BBB/CCC"
  slack_channel     = "main-aws-notification"
  slack_username    = "AWS"
}
