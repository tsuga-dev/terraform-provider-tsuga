# Terraform Tsuga Provider

This repository has Tsuga's Terraform provider. We aim at automatically generating this code as much as possible based on our OpenAPI spec.

To that end, read the "Updating" section.

## Updating

**Important:** Always follow all steps here.

First ensure that the repository has an up-to-date version of the OpenAPI spec.

Then run the codegen script:

```bash
sh codegen.sh
```

Note: the codegen script will only update the code for resources defined in `generator_config.yml`. For other resources, you will need to update the code manually.

Remember to update the examples folder too.

Finally run the `docgen.sh` script, so that any examples and documentation that you included in the code be injected in the documentation.

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
