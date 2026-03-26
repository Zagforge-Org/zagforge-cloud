output "service_url" {
  value = google_cloud_run_v2_service.auth.uri
}

output "service_account_email" {
  value = google_service_account.auth.email
}
