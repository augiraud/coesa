# Cloud Run setup

The `gcloud` commands behind the repository scripts, in the order a project that has never hosted this service needs them. The values come from `.env` at the repository root; see [the README](../README.md#container-image) for the variables and `.env.template` for the shape of the file.

## Project and repository

```sh
. ./.env
```

Loads the variables the remaining commands and the repository scripts use.

```sh
gcloud services enable run.googleapis.com artifactregistry.googleapis.com --project "$GOOGLE_CLOUD_PROJECT"
```

Enables the two APIs that everything else depends on.

```sh
gcloud artifacts repositories create "$ARTIFACT_REGISTRY_REPOSITORY" \
  --repository-format=docker \
  --location="$GOOGLE_CLOUD_REGION" \
  --project "$GOOGLE_CLOUD_PROJECT"
```

Creates the Docker-format repository that `scripts/push_docker_image_to_gcloud.sh` pushes to. A repository in the same project needs no extra grant for Cloud Run to pull from it; a repository in another project needs `roles/artifactregistry.reader` for the identity that performs the pull.

## Docker authentication

```sh
gcloud auth configure-docker "$GOOGLE_CLOUD_REGION-docker.pkg.dev" --quiet
```

Registers `gcloud` as the Docker credential helper for that registry host by writing a `credHelpers` entry into Docker's configuration file, so `docker push` authenticates as your active `gcloud` account. The `--quiet` flag skips the confirmation prompt.

## Runtime identity

```sh
gcloud iam service-accounts create "$CLOUD_RUN_SERVICE_ACCOUNT" \
  --project "$GOOGLE_CLOUD_PROJECT"
```

Creates the identity the container runs as. Grant it nothing: the server calls no Google API, so an account with no roles is the correct shape. Naming it explicitly also keeps the service off the project's default compute service account, which Google commonly creates with `roles/editor`, and an unauthenticated public endpoint holding project-wide write credentials is worth avoiding even for a demonstration.

```sh
gcloud projects get-iam-policy "$GOOGLE_CLOUD_PROJECT" \
  --flatten="bindings[].members" \
  --filter="bindings.members:$CLOUD_RUN_SERVICE_ACCOUNT@" \
  --format="table(bindings.role)"
```

Lists any role granted to that account anywhere in the project. No output rows is the pass condition.

## Build, push, deploy

Commit before building: `scripts/build_docker.sh` refuses a dirty working tree because it tags the image `coesa:$(git rev-parse --short HEAD)`, and that tag is only meaningful if it names a commit.

```sh
./scripts/build_docker.sh
./scripts/push_docker_image_to_gcloud.sh
./scripts/deploy_cloud_run.sh
```

The first builds the image for `linux/amd64`, which is what Cloud Run runs, and tags it with both the short commit and the local convenience tag `coesa:latest` that `docker-compose.yml` uses. The second re-tags that image for the repository and pushes it under the commit tag. The third creates or updates the Cloud Run service with the flags written in the script: first-generation environment, 0.08 vCPU, 256 MiB, concurrency 1, scaling between zero and one instance, a 30 second request timeout, the service account above, and unauthenticated invocation. All three read the image reference from `scripts/image_ref.sh`, so the name is written once.

## Check it

```sh
gcloud run services describe coesa \
  --project "$GOOGLE_CLOUD_PROJECT" --region "$GOOGLE_CLOUD_REGION" \
  --format="value(status.url, spec.template.spec.serviceAccountName)"
```

Prints the service URL and confirms which identity the service runs as.

```sh
./scripts/query_cloud_run.sh 10000
```

Looks up the service URL and queries it. Should answer `223.25… K` and `26499.9… Pa`, the published values for 10 km. Without an argument the script queries 11,000 m.

```sh
gcloud artifacts docker tags list \
  "$GOOGLE_CLOUD_REGION-docker.pkg.dev/$GOOGLE_CLOUD_PROJECT/$ARTIFACT_REGISTRY_REPOSITORY" \
  --project "$GOOGLE_CLOUD_PROJECT"
```

Lists the commit tags in the repository with their digests. Cloud Run pins each revision to a digest, so `gcloud run revisions describe` tells you which image a running revision actually uses even after `latest` has moved on.

## Removing the service

```sh
gcloud run services delete coesa \
  --project "$GOOGLE_CLOUD_PROJECT" --region "$GOOGLE_CLOUD_REGION" --quiet
```

Removes the service. This is optional rather than needed for cost: with a minimum of zero instances an idle service accrues no instance charges. Artifacts continue to occupy repository storage, which is free up to 0.5 GiB.
