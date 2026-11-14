# Cloud Run setup for the demo service

These are the `gcloud` commands behind the `cmd/coesa` demonstration service, in the order a fresh project needs them. The values come from `.env` at the repository root, described in [the README](../README.md#container-image). The Artifact Registry repository and the service account already existed in this project, so the creation commands below were not re-run here; the flag names were checked against Google Cloud SDK 593.0.0.

## Project and repository

```sh
. ./.env
```

Loads the four variables the remaining commands and the repository scripts use.

```sh
gcloud services enable run.googleapis.com artifactregistry.googleapis.com --project "$GOOGLE_CLOUD_PROJECT"
```

Enables the two APIs everything else depends on.

```sh
gcloud artifacts repositories create "$ARTIFACT_REGISTRY_REPOSITORY" \
  --repository-format=docker \
  --location="$GOOGLE_CLOUD_REGION" \
  --project "$GOOGLE_CLOUD_PROJECT"
```

Creates the Docker-format repository that `scripts/push_docker_image_to_gcloud.sh` pushes to. Cloud Run pulls from a repository in the same project without any extra grant; a cross-project repository would need `roles/artifactregistry.reader` for the pulling identity.

## Docker authentication

```sh
gcloud auth configure-docker "$GOOGLE_CLOUD_REGION-docker.pkg.dev" --quiet
```

Registers `gcloud` as the Docker credential helper for that registry host by writing a `credHelpers` entry into `~/.docker/config.json`, so `docker push` authenticates as your active `gcloud` account. The `--quiet` flag skips the confirmation prompt.

## Runtime identity

```sh
gcloud iam service-accounts create "$CLOUD_RUN_SERVICE_ACCOUNT" \
  --project "$GOOGLE_CLOUD_PROJECT"
```

Creates the identity the container runs as. Grant it nothing: the server calls no Google API, so an account with no roles is the correct shape, and it keeps the service off the project's default compute account, which carries `roles/editor` in this project.

```sh
gcloud projects get-iam-policy "$GOOGLE_CLOUD_PROJECT" \
  --flatten="bindings[].members" \
  --filter="bindings.members:$CLOUD_RUN_SERVICE_ACCOUNT@" \
  --format="table(bindings.role)"
```

Lists any role granted to that account anywhere in the project; no output rows is the pass condition.

## Build, push, deploy

```sh
./scripts/build_docker.sh
./scripts/push_docker_image_to_gcloud.sh
./scripts/deploy_cloud_run.sh
```

The first builds `coesa:latest` locally, the second re-tags it for the repository and pushes it, and the third creates or updates the Cloud Run service with the flags written in the script: first-generation environment, 0.08 vCPU, 256 MiB, concurrency 1, scaling between zero and one instance, a 30 second request timeout, the service account above, and unauthenticated invocation.

## Check it

```sh
gcloud run services describe coesa \
  --project "$GOOGLE_CLOUD_PROJECT" --region "$GOOGLE_CLOUD_REGION" \
  --format="value(status.url, spec.template.spec.serviceAccountName)"
```

Prints the service URL and confirms which identity it runs as.

```sh
curl "<status.url>/v1/coesa?altitude_m=10000"
```

Should answer `223.25… K` and `26499.9… Pa`, the published values for 10 km.

```sh
gcloud artifacts docker tags list \
  "$GOOGLE_CLOUD_REGION-docker.pkg.dev/$GOOGLE_CLOUD_PROJECT/$ARTIFACT_REGISTRY_REPOSITORY" \
  --project "$GOOGLE_CLOUD_PROJECT"
```

Shows the digest the `latest` tag points at in the repository, which is how to confirm a push landed.

## When the demo is over

```sh
gcloud run services delete coesa \
  --project "$GOOGLE_CLOUD_PROJECT" --region "$GOOGLE_CLOUD_REGION" --quiet
```

Removes the service. This is optional rather than required for cost: with a minimum of zero instances an idle service accrues no instance charges, though the image continues to occupy repository storage, which is free up to 0.5 GiB.
