# Publishing

We're publishing docker images and helm charts over Github Actions. The Github Actions will run these two jobs on master branch.

The Github Action job needs the following credentials defined in "Repository Settings":

```bash
DOCKER_IMAGE_ORG="DOCKER IMAGE ORGANIZATION"
DOCKER_USERNAME="DOCKER USERNAME"
DOCKER_PASSWORD="DOCKER PASSWORD"
S3_BUCKET="PUBLISH S3 BUCKET"
AWS_ACCESS_KEY="YOUR AWS ACCESS KEY"
AWS_SECRET_KEY="YOUR AWS SECRET KEY"
```
