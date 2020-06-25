# QuarksJob

[![godoc](https://godoc.org/code.cloudfoundry.org/quarks-job?status.svg)](https://godoc.org/code.cloudfoundry.org/quarks-job)
[![](https://github.com/cloudfoundry-incubator/quarks-job/workflows/quarks-job-ci/badge.svg?branch=master)](https://github.com/cloudfoundry-incubator/quarks-job/actions?query=branch%3Amaster)
[![go report card](https://goreportcard.com/badge/code.cloudfoundry.org/quarks-job)](https://goreportcard.com/report/code.cloudfoundry.org/quarks-job)
[![Coveralls github](https://img.shields.io/coveralls/github/cloudfoundry-incubator/quarks-job.svg?style=flat)](https://coveralls.io/github/cloudfoundry-incubator/quarks-job?branch=HEAD)

- [QuarksJob](#quarksjob)
  - [Description](#description)
  - [Features](#features)
    - [Errand Jobs](#errand-jobs)
    - [One-Off Jobs / Auto-Errands](#one-off-jobs--auto-errands)
      - [Restarting on Config Change](#restarting-on-config-change)
    - [Persisted Output](#persisted-output)
      - [Versioned Secrets](#versioned-secrets)
  - [`QuarksJob` Examples](#quarksjob-examples)

## Description

An `QuarksJob` allows the developer to run jobs when something interesting happens. It also allows the developer to store the output of the job into a `Secret`.
The job started by an `QuarksJob` is deleted automatically after it succeeds.

There are two different kinds of `QuarksJob`:

- **one-offs**: automatically runs once after it's created
- **errands**: needs to be run manually by a user

## Features

### Errand Jobs

Errands are run manually by the user. They are created by setting `trigger.strategy: manual`.

After the `QuarksJob` is created, run an errand by editing and applying the
manifest, i.e. via `kubectl edit errand1` and change `trigger.strategy: manual` to `trigger.strategy: now`. A `kubectl patch` is also a good way to trigger this type of `QuarksJob`.

After completion, this value is reset to `manual`.

Look [here](https://github.com/cloudfoundry-incubator/quarks-job/blob/master/docs/examples/qjob_errand.yaml) for a full example of an errand.

### One-Off Jobs / Auto-Errands

One-off jobs run directly when created, just like native k8s jobs.

They are created with `trigger.strategy: once` and switch to `done` when
finished.

If a versioned secret is referenced in the pod spec of an qJob, the most recent
version of that secret will be used when the batchv1.Job is created.

#### Restarting on Config Change

A **one-off** `QuarksJob` can
automatically be restarted if its environment/mounts have changed, due to a
`ConfigMap` or a `Secret` being updated. This also works for [Versioned Secrets](#versioned-secrets).

This requires the attribute `updateOnConfigChange` to be set to true.

Once `updateOnConfigChange` is enabled, modifying the `data` of any `ConfigMap` or `Secret` referenced by the `template` section of the job will trigger the job again.

### Persisted Output

QuarksJob can create secrets from job output, which is written to a JSON file in `/mnt/quarks`.

Multiple secrets are created or overwritten per container in the pod. The output file names are mapped to the secrets' names via `OutputMap`. This is done for every container.

The only supported output type currently is json with a flat structure, i.e.
all values being string values, because [Kubernetes secrets store base64 encoded data](https://kubernetes.io/docs/concepts/configuration/secret/#creating-a-secret-manually). The string value can be a marshalled JSON document.

**Note:** Output of previous runs is overwritten.

The behavior of storing the output is controlled by specifying the following parameters:

- `outputMap` - Mapping from output file name to the name of the secret(s) that will hold the output.
- `outputType` - Currently only `json` is supported. (default: `json`)
- `secretLabels` - An optional map of labels which will be attached to the generated secret(s)
- `writeOnFailure` - if true, output is written even though the Job failed. (default: `false`)

The developer should ensure that she creates all files defined in `OutputMap` in the /mnt/quarks volume mount at the end of the container script. An example of the command field in the quarks job spec will look like this

```
command: ["/bin/sh"]
args: ["-c","json='{\"foo\": \"1\", \"bar\": \"baz\"}' && echo $json >> /mnt/quarks/output.json"]
```

The secret is created by a side container in quarks job pod which captures the create event of /mnt/quarks/output.json file.

The behavior of storing the output is controlled by specifying the following parameters:

  - `outputType` - Currently only `json` is supported. (default: `json`)
  - `secretLabels` - An optional map of labels which will be attached to the generated secret(s)
  - `versioned` - if true, the output is written in a [Versioned Secret](#versioned-secrets)
  - `writeOnFailure` - if true, output is written even though the Job failed. (default: `false`)

#### Versioned Secrets

Versioned Secrets are a set of `Secrets`, where each of them is immutable, and contains data for one iteration. Implementation can be found in the [versionedsecretstore](https://github.com/cloudfoundry-incubator/quarks-utils/tree/master/pkg/versionedsecretstore) package.

When an `QuarksJob` is configured to save to "Versioned Secrets", the controller looks for the `Secret` with the largest ordinal, adds `1` to that value, and _creates a new Secret_.

Each versioned secret has the following characteristics:

- its name is calculated like this: `<name>-v<ORDINAL>` e.g. `mysecret-v2`
- it has the following labels:
  - `quarks.cloudfoundry.org/secret-kind` with a value of `versionedSecret`
  - `quarks.cloudfoundry.org/secret-version` with a value set to the `ordinal` of the secret
- an annotation of `quarks.cloudfoundry.org/source-description` that contains arbitrary information about the creator of the secret

## `QuarksJob` Examples

See https://github.com/cloudfoundry-incubator/quarks-job/tree/master/docs/examples
