# Listing requires password authentication.
data "kintone_apps" "example" {
  name = "Customer"
}

output "matching_apps" {
  value = data.kintone_apps.example.apps
}
