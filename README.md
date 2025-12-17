# Terraform Tsuga Provider

This repository has Tsuga's Terraform provider. We aim at automatically generating this code as much as possible based on our OpenAPI spec.

To that end, we have a `codegen.sh` script which automatically generates part of this codebase.

## Code Generation

To regenerate the provider code from the OpenAPI spec, run:

```bash
sh codegen.sh
```

**Important:** Always run this script and commit the generated files when you update the OpenAPI spec.

## Example Validation

This repository includes automated validation of all Terraform examples to ensure they remain valid.

## Continuous Integration

The GitHub Actions CI pipeline includes:

- **Codegen Check**: Verifies that the codegen script has been run and all generated code is up-to-date
- **Terraform Validation**: Builds the provider and validates the Terraform configuration
- **Go Tests**: Runs Go tests and static analysis
- **Example Validation**: Validates that all examples are syntactically correct

All checks must pass before merging pull requests.

## Resources

- Terraform code generation tutorial: https://developer.hashicorp.com/terraform/plugin/code-generation/workflow-example
- Terraform spec generator from OpenAPI: https://developer.hashicorp.com/terraform/plugin/code-generation/openapi-generator#installation
- Terraform code generator: https://developer.hashicorp.com/terraform/plugin/code-generation/framework-generator#installation