resource "kintone_app" "example" {
  name        = "Customer requests"
  description = "Managed with Terraform"
  theme       = "BLUE"

  title_field = {
    selection_mode = "AUTO"
    # Omit field_code: state contains the code selected by kintone.
  }

  number_precision = {
    total_digits   = 16
    decimal_places = 4
    rounding_mode  = "HALF_EVEN"
  }
  comments_enabled = true
}

# Omitted settings retain their existing values. Do not leave unrelated preview
# changes on this app: deployment includes them. Destroy removes only Terraform
# state; app deletion requires manual cleanup in kintone.
