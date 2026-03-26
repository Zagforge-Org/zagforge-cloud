output "api_job_name" {
  value = google_cloud_run_v2_job.migrate.name
}

output "auth_job_name" {
  value = google_cloud_run_v2_job.auth_migrate.name
}
