# Scripts Directory

This directory contains utility scripts used for development, testing, and
CI/CD operations for the Kubewarden project.

The scripts in this directory serve various purposes:

- **Validation and Testing:** Ensure consistency and correctness of project
  artifacts
- **Chart Management:** Helper scripts for Helm chart operations
- **Policy Operations:** Tools for managing Kubewarden policies
- **Maintenance:** Scripts for cleanup and uninstallation tasks

## Scripts

### Hauler Manifest Validation

**Script:** `validate-hauler-manifest.sh`

This script validates that the Hauler manifest (`charts/hauler.yaml`) stays in
sync with Helm chart definitions. It compares versions of container images and
Helm charts between the chart definitions and the Hauler manifest to prevent
version mismatches that could cause issues in air-gapped deployments.

The script runs automatically in CI when the `ci-full` label is added to a PR,
on pushes to the main branch, and on manual workflow triggers. It validates all
container images (kubewarden-controller, audit-scanner, policy-server,
kuberlr-kubectl, and policy modules) and Helm charts (kubewarden-crds,
kubewarden-controller, kubewarden-defaults, policy-reporter, openreports).

The weekly updatecli workflow automatically updates both Helm chart values and
the Hauler manifest. This validation serves as a safety check to catch any
manual changes or update failures.
