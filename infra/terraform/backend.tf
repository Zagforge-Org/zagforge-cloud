terraform {
  backend "gcs" {
    bucket = "zagforge-org-terraform-state"
  }
}
