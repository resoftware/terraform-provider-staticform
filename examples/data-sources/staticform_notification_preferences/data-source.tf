data "staticform_notification_preferences" "me" {}

output "notification_event_keys" {
  value = [for e in data.staticform_notification_preferences.me.events : e.key]
}
