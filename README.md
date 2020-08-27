# QuarksJob

[![godoc](https://godoc.org/code.cloudfoundry.org/quarks-job?status.svg)](https://godoc.org/code.cloudfoundry.org/quarks-job)
[![](https://github.com/cloudfoundry-incubator/quarks-job/workflows/quarks-job-ci/badge.svg?branch=master)](https://github.com/cloudfoundry-incubator/quarks-job/actions?query=branch%3Amaster)
[![go report card](https://goreportcard.com/badge/code.cloudfoundry.org/quarks-job)](https://goreportcard.com/report/code.cloudfoundry.org/quarks-job)
[![Coveralls github](https://img.shields.io/coveralls/github/cloudfoundry-incubator/quarks-job.svg?style=flat)](https://coveralls.io/github/cloudfoundry-incubator/quarks-job?branch=HEAD)

<img align="right" width="200" height="39" src="https://github.com/cloudfoundry-incubator/quarks-docs/raw/master/content/en/docs/cf-operator-logo.png">

----

`QuarksJob` is part of Project Quarks. It's used by the quarks-operator

A `QuarksJob` allows the developer to run jobs when something interesting happens. It also allows the developer to store the output of the job into a `Secret`.
The job started by an `QuarksJob` is deleted automatically after it succeeds.

[See the official documentation for more informations](https://quarks.suse.dev/docs/quarks-job/)

----


* Incubation Proposal: [Containerizing Cloud Foundry](https://docs.google.com/document/d/1_IvFf-cCR4_Hxg-L7Z_R51EKhZfBqlprrs5NgC2iO2w/edit#heading=h.lybtsdyh8res)
* Slack: #quarks-dev on <https://slack.cloudfoundry.org>
* Backlog: [Pivotal Tracker](https://www.pivotaltracker.com/n/projects/2192232)
* Docker: https://hub.docker.com/r/cfcontainerization/cf-operator/tags
* Documentation: [quarks.suse.dev](https://quarks.suse.dev)
