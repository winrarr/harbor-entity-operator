# Releases

The published artifacts are currently stored in GHCR. A release branch carries
the release metadata for both artifacts:

- `charts/harbor-entity-operator/Chart.yaml:appVersion` selects the operator image;
- `charts/harbor-entity-operator/Chart.yaml:version` selects the Helm chart package.

The versions are allowed to differ. For example, a release can publish
operator `0.7.1` in chart version `0.5.1`.

## Release tags

Release tags are the publication interface:

- `vX.Y.Z` or `vX.Y.Z-rc.N` publishes the operator image and a matching chart.
- `chart-vX.Y.Z` or `chart-vX.Y.Z-rc.N` publishes a chart-only release.

The publisher packages the chart from the same tagged commit as the image and
attaches the chart package to the GitHub Release for `v*` tags. Do not create a
separate `chart-v*` tag for the same paired operator release.

For `chart-v*` tags, the publisher verifies that the image selected by
`appVersion` already exists before publishing the chart.

The publisher also supports manual dispatch with an existing tag name for
recovery. Re-run or dispatch the workflow for the existing tag; do not move or
recreate a release tag.

## Create a release branch

Create release branches manually from the intended source commit:

```sh
git switch main
git pull --ff-only
git switch -c release/v0.7
```

Set the initial operator and chart versions in
`charts/harbor-entity-operator/Chart.yaml`, commit the metadata, and push the branch:

```sh
git add charts/harbor-entity-operator/Chart.yaml
git commit -m "chore(release): prepare v0.7.0"
git push --set-upstream origin release/v0.7
```

New minor release branches use an operator version ending in `.0`. The
operator version line must match the branch name. The chart version may use a
different line.

## Paired operator and chart release

Update both `version` and `appVersion` on the release branch, then commit and
push the change. Wait for these checks on that commit:

- `docs`
- `lint`
- `verify-generated`
- `test`
- `test-e2e`

Create an annotated operator tag from the checked commit:

```sh
git tag -a v0.7.1 -m "Release v0.7.1"
git push origin v0.7.1
```

The `Publish Release` workflow then:

1. validates that `appVersion` is `0.7.1`;
2. builds and pushes `ghcr.io/winrarr/harbor-entity-operator:0.7.1`;
3. packages and pushes the chart using `Chart.yaml.version`;
4. creates the GitHub Release and uploads the chart package.

Stable operator releases use stable chart versions. Release candidates use
matching release-candidate metadata.

## Chart-only release

For a chart-only change, update the chart templates or values and increment
only `Chart.yaml.version`. Keep `appVersion` set to the existing operator
image version. After the required checks pass, create the chart tag:

```sh
git tag -a chart-v0.5.2 -m "Chart release chart-v0.5.2"
git push origin chart-v0.5.2
```

The publisher verifies that the image selected by `appVersion` exists and
publishes the chart version from `Chart.yaml`.

## Dependency-only patch train

The patch train runs for dependency-only changes to `go.mod`, `go.sum`, or
`Dockerfile` on supported `release/vX.Y` branches. It also runs weekly as a
recovery backstop and can be run manually for a selected branch.

For an eligible branch, it:

1. increments the operator patch version and chart patch version;
2. updates both values in `Chart.yaml`;
3. commits and pushes the metadata change;
4. waits for the required checks;
5. creates one annotated `vX.Y.Z` tag.

That operator tag triggers the same paired publication workflow as a manual
release, so the patch train publishes both the operator image and chart.

Chart changes outside `Chart.yaml` require explicit chart-release intent and
block the dependency-only patch train until they have been released.

The implementation lives in `hack/release_branch_patch_train.py`.
Run `python3 hack/test_release_branch_patch_train.py` when changing its
behavior.

## Recovery and release notes

If a tag exists but publication failed, re-run the failed `Publish Release`
workflow. If no workflow run exists, dispatch it with the existing tag:

```sh
gh workflow run publish-release.yaml -f tag_name=v0.7.1
```

To validate a tag through GitHub without publishing anything, use the workflow's dry-run mode. It still validates the tag, waits for required checks, builds the operator image, and packages the chart:

```sh
gh workflow run publish-release.yaml -f tag_name=v0.12.1 -f dry_run=true
```

Before recovery, inspect the tag, release branch, `Chart.yaml`, GitHub Release,
and published assets. Existing image and chart versions must not be replaced
with different content.

Operator releases determine GitHub's `latest` stable release. Chart-only
releases never become `latest`. Release notes compare only with the previous
tag in the same family.
