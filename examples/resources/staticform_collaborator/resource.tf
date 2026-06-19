resource "staticform_collaborator" "editor" {
  form_id              = staticform_form.contact.id
  email                = "colleague@example.com"
  can_view_form        = true
  can_edit_form        = true
  can_view_submissions = true
}
