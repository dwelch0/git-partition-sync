# Git Partition Sync - Producer
Utility for cloning specified source git repositories and uploading encrypted/tarred objects to s3. Uploaded objects are consumed by [Git Partition Sync - Consumer](https://github.com/dwelch0/git-partition-sync/consumer)

## Environment Variables

### Required
* AWS_ACCESS_KEY_ID - s3 CRUD permissions required
* AWS_SECRET_ACCESS_KEY
* AWS_REGION
* AWS_S3_BUCKET - the name. not an ARN
* CONFIG_FILE_PATH - absolute path to yaml config file 
* GITLAB_BASE_URL - GitLab instance base url. Ex: https://gitlab.foobar.com
* GITLAB_USERNAME
* GITLAB_TOKEN - repository read permission required
* PUBLIC_KEY - value of x25519 format public key. See [age encryption](https://github.com/FiloSottile/age#readme)

### Optional
* RECONCILE_SLEEP_TIME - time between runs. defaults to 5 minutes (5m)
* WORKDIR - local directory where io operations will be performed

## Config File Format
Yaml file is an array of objects with following format:

```
source: 
  project_name
  namespace
  branch
destination:
  project_name
  namespace
  branch
```

## Uploaded s3 Object Key Format
Uploaded keys are base64 encoded. Decoded, the key is a json string with following structure:
```
{
  "group":"some-gitlab-group",
  "project_name":"some-gitlab-project",
  "commit_sha":"full-commit-sha",
  "branch":"master"
}
```
**Note:** the values within each json will mirror values for each `destination` defined within config file (exluding `commit_sha` which is the latest commit pulled from `source`)

**Note:** only the latest version for a repository will be stored within s3. If `producer` detects a newer commit on a target repository, once that new version is uploaded, the old object will be deleted from the bucket. Additionally, if a project is removed entirely from the config file, `producer` will delete the object upon next execution.

## Execute
Ensure outputted operations are desired:
```
docker run -t \
    -e AWS_ACCESS_KEY_ID="$AWS_ACCESS_KEY_ID" \
    -e AWS_SECRET_ACCESS_KEY="$AWS_SECRET_ACCESS_KEY" \
    -e AWS_REGION="$AWS_REGION" \
    -e AWS_S3_BUCKET="$AWS_S3_BUCKET" \
    -e GITLAB_BASE_URL="$GITLAB_BASE_URL" \
    -e GITLAB_USERNAME="$GITLAB_USERNAME" \
    -e GITLAB_TOKEN="$GITLAB_TOKEN" \
    -e PUBLIC_KEY="$PUBLIC_KEY" \
    quay.io/app-sre/git-partition-sync-producer:latest -dry-run
```
If operations look correct, run again without `-dry-run`