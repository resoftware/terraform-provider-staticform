data "staticform_forms" "all" {}

output "form_names" {
  value = [for f in data.staticform_forms.all.forms : f.name]
}
