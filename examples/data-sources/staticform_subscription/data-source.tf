data "staticform_subscription" "current" {}

output "plan_status" {
  value = data.staticform_subscription.current.active ? data.staticform_subscription.current.status : "none"
}
