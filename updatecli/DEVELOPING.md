# Release automation

You can trigger the pipeline and open the PR from your local machine
as needed.

Change `updatecli/values.yaml` as needed to target forks.

```console
$ cd kubewarden-controller/
$ export UPDATECLI_GITHUB_TOKEN=<your token>
$ clear; updatecli apply --config updatecli/updatecli.release.d/open-release-pr.yaml \
  --values updatecli/values.yaml \
  --debug --clean=true

(...)

Run Summary
===========
Pipeline(s) run:
  * Changed:    1
  * Failed:     0
  * Skipped:    0
  * Succeeded:  0
  * Total:      1

One action to follow up:
  * https://github.com/viccuad/kubewarden-controller/pull/1
```

## Dependency Updates

There are separate updatecli configurations for different types of dependency updates:

### Golang Version Updates
Runs hourly via `.github/workflows/updatecli.yaml` using `updatecli-compose.yaml`.
Updates Go version in `go.mod` and Dockerfiles.

### Chart Dependencies Updates
Runs weekly (Monday 3:30) via `.github/workflows/update-dependencies.yaml` using `update-deps.yaml`.
Updates:
- Policy image tags
- Policy-reporter chart version
- Kuberlr-kubectl image
- Hauler manifest

### Running All Updates Together
For manual runs that combine both golang and chart updates:
```console
$ export UPDATECLI_GITHUB_TOKEN=<your token>
$ updatecli compose diff --file updatecli/updatecli-compose-all.yaml
```
