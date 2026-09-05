data "kintone_app" "example" {
  id = "123"
}

output "app_title_field" {
  value = data.kintone_app.example.title_field
}
