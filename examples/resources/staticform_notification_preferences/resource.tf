resource "staticform_notification_preferences" "me" {
  email_mode        = "DailyDigest"
  digest_hour_local = 8
  time_zone_id      = "Europe/Amsterdam"

  quiet_hours = {
    start = "22:00"
    end   = "07:00"
  }

  events = {
    # Formerly submit_action_failure_notification_enabled
    "product.SubmitActionFailed" = { in_app = true, email = true }

    # Formerly email_quota_warning_enabled
    "product.EmailQuotaWarning" = { in_app = true, email = true }

    # Formerly storage_warning_enabled
    "product.StorageWarning" = { in_app = true, email = false }

    # Keep new-submission notices in the inbox only
    "product.SubmissionReceived" = { email = false }
  }
}
