# CI Credentials (Google Secret Manager)

How the Quay pull credentials used by the Prow CI jobs are stored, delivered, and
rotated.

## Prow does not read Vault

OpenShift CI migrated secret storage to Google Secret Manager (GSM) in September 2026.
`vault.ci.openshift.org` is read-only and deprecated. **Updating Vault has no effect on
CI.**

The Vault values for this project were copied into GSM once, on 2026-09-10, during the
bulk migration. That was a one-time snapshot, not a sync — Vault and GSM have been
independent ever since. A Vault entry can show version 8 while GSM still serves the
original version 1.

This caused a three-week CI outage in October 2026: the `acm-d` robot token was rotated
upstream around 15-16 October, the new value was written to Vault, GSM kept serving the
dead September copy, and every job failed with `ErrImagePull`.

## Where the credentials live

| Property | Value |
|----------|-------|
| GCP project | `openshift-ci-secrets` |
| Collection | `stolostron-capi-tests` |
| Owning Rover group | `aro-hcp-capz` |
| Mount path in CI pods | `/etc/quay-pull-credentials` |

Ownership is declared in `core-services/sync-rover-groups/_config.yaml` in
[openshift/release](https://github.com/openshift/release).

Fields in the collection:

| GSM field | Consumed by CI? |
|-----------|-----------------|
| `--dot--dockerconfigjson` | **Yes** — the only field any step reads |
| `acm_d_mce_quay_username` | No — leftover from the Vault era |
| `acm_d_mce_quay_pullsecret` | No — leftover from the Vault era |
| `auth` | No — leftover from the Vault era |

## Delivery path

1. The `credentials:` block in the write-env step ref mounts the collection via CSI at
   pod start, resolving `/versions/latest`.
2. The step reads `/etc/quay-pull-credentials/.dockerconfigjson`, base64-encodes it into
   `IMAGE_PULL_SECRET_B64`, and writes it to `${SHARED_DIR}/capz-test-env.sh`.
3. Later steps source that generated file; the Helm charts use the value to create the
   `capi-manager-pull-credentials` secret in the management cluster.

Secrets exist only for the lifetime of the pod. They are not pre-synced into clusters —
the collection is deliberately absent from `ci-secret-bootstrap/gsm-config.yaml`.

## Rotating the Quay pull credentials

Required whenever the `acm-d` robot token is rotated upstream.

**Symptom of a missed rotation:** controller pods in `ErrImagePull` / `ImagePullBackOff`
and `TestKindCluster_CAPIControllerReady` failing within seconds.

### 1. Verify the new credential before uploading it

Save it to a file, then check all four images actually used by the jobs:

```bash
skopeo inspect --authfile /path/to/dockerconfigjson --no-tags \
  docker://quay.io/acm-d/ose-cluster-api-rhel9:v5.0
skopeo inspect --authfile /path/to/dockerconfigjson --no-tags \
  docker://quay.io/acm-d/mce-capi-webhook-config-rhel9:5.0-dev
skopeo inspect --authfile /path/to/dockerconfigjson --no-tags \
  docker://quay.io/acm-d/cluster-api-provider-azure-rhel9:5.0-dev
skopeo inspect --authfile /path/to/dockerconfigjson --no-tags \
  docker://quay.io/acm-d/azure-service-operator-rhel9:5.0-dev
```

All four must return a digest. This check takes seconds and distinguishes a bad
credential from a bad delivery path — do it before changing anything.

### 2. Upload a new version to GSM

```bash
gcloud secrets versions add \
  stolostron-capi-tests__quay-pull-credentials__--dot--dockerconfigjson \
  --project=openshift-ci-secrets \
  --data-file=/path/to/dockerconfigjson
```

The supported alternative is the `sm` CLI from `openshift/ci-tools` — see the
[Secret Manager CLI docs](https://docs.ci.openshift.org/architecture/cli-secret-manager/).

### 3. Confirm the new version is live

```bash
gcloud secrets versions list \
  stolostron-capi-tests__quay-pull-credentials__--dot--dockerconfigjson \
  --project=openshift-ci-secrets
```

The newest version must be `enabled`.

**Jobs resolve `latest` at pod start, so only runs started after the upload pick it up.**
A job triggered minutes earlier still mounts the old version and fails identically — do
not read that as the fix having failed.

### 4. Roll back if needed

Disable the new version; `latest` falls back to the previous enabled one.

```bash
gcloud secrets versions disable <N> \
  --secret=stolostron-capi-tests__quay-pull-credentials__--dot--dockerconfigjson \
  --project=openshift-ci-secrets
```

## Access requirements

Writing requires membership of the `aro-hcp-capz` Rover group, which owns the collection.

Two quirks worth knowing:

- `gcloud secrets list` is **denied** at project level even for members. This is expected
  and does not mean you lack access. Use
  `gcloud secrets versions list <full-secret-name>` instead, which works.
- Members can read version *metadata* but **cannot read secret values back out of GSM**.
  Keep a copy wherever your team stores credentials for human use.

## Verifying a fix end to end

A successful write-env step logs:

```text
[quay-diag] quay.io/acm-d/ose-cluster-api-rhel9:v5.0 manifest access: SUCCESS
```

for each of the four images. That proves the Prow step can authenticate. It does **not**
prove the node-side pulls succeed — wait for the suite to get past
`TestKindCluster_CAPIControllerReady` before declaring the job healthy.

## Staying informed

- **`#forum-ocp-testplatform`** (Slack) — where DPTP announces deprecations and
  migrations. The Vault-to-GSM move was announced here through early October 2026.
- [Adding a new secret to CI (GSM)](https://docs.ci.openshift.org/how-tos/adding-a-new-secret-to-ci-gsm/)
- [Secret Manager CLI](https://docs.ci.openshift.org/architecture/cli-secret-manager/)
- Watch for the collection appearing under `test-platform-gsm-unclaimed-secrets` in
  `core-services/sync-rover-groups/_config.yaml`. `unclaimed: true` means no team owns
  it and nobody can update it — this state persisted unnoticed for seven weeks in 2026.
