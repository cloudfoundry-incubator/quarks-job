# Publishing

We're publishing docker images and helm charts over travis. The travis will run these two jobs on master branch.

The travis needs following credentials variables to define variables in Repository Settings:

```bash
DOCKER_IMAGE_ORG="DOCKER IMAGE ORGANIZATION"
DOCKER_USERNAME="DOCKER USERNAME"
DOCKER_PASSWORD="DOCKER PASSWORD"
S3_BUCKET="PUBLISH S3 BUCKET"
AWS_ACCESS_KEY="YOUR AWS ACCESS KEY"
AWS_SECRET_KEY="YOUR AWS SECRET KEY"
```
