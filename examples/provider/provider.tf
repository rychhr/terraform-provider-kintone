terraform {
  required_version = ">= 1.16.0"

  required_providers {
    kintone = {
      source = "rychhr/kintone"
    }
  }
}

provider "kintone" {
  # Set KINTONE_BASE_URL, KINTONE_USERNAME, and KINTONE_PASSWORD in the environment.
  # For token-only access to existing apps, use KINTONE_API_TOKENS instead.
}
