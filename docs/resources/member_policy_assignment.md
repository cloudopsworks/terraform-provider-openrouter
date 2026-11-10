# openrouter_member_policy_assignment

Manages one direct organization member assignment to a workspace policy.

## Example Usage

```hcl
resource "openrouter_member_policy_assignment" "service" {
  workspace_id = openrouter_workspace.team.id
  policy_type  = "budget"
  policy_id    = openrouter_budget_policy.service.id
  member_id    = var.member_id
}
```

Use an existing organization member user ID. This resource does not create a member or manage workspace membership.

## Argument Reference

All arguments are required and force replacement when changed:

- `workspace_id` - Workspace UUID.
- `policy_type` - Policy type: `budget`, `model_availability`, or `moderation`.
- `policy_id` - Native, non-default policy UUID.
- `member_id` - Subject identifier.

## Attribute Reference

- `id` - Composite identity: `<workspace_id>/<policy_type>/<subject_id>/<policy_id>`.

## Ownership and Concurrency

Manage each subject/type/workspace binding with exactly one Terraform resource. Creation refuses an existing direct binding, including a binding to the requested policy; import that binding instead.

Mutations send the complete assignment state just read from OpenRouter as `expected_assignment`. HTTP 409 reports concurrent modification; the provider does not retry blindly or overwrite a newly observed owner. Refresh removes the resource from state when its direct binding disappears. Delete does not remove another policy's binding.

Only direct assignments to native, non-default policies are managed. The resource does not manage inherited default assignments or create default opt-outs. It rejects unsafe default/opt-out states rather than interpreting an inherited control as a removable direct binding. Resolve those states outside this resource before retrying.

Avoid overlapping ownership with legacy guardrail assignments or bulk policy assignment updates.

## Import

```sh
terraform import openrouter_member_policy_assignment.service '<workspace_uuid>/<policy_type>/<subject_id>/<policy_uuid>'
```

Import verifies that the requested direct binding exists and that the policy is eligible for this lifecycle.
