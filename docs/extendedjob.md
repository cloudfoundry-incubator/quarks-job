# ExtendedJob

1. [ExtendedJob](#extendedjob)
   1. [Description](#description)
   2. [ExtendedJob Component](#extendedjob-component)
      1. [Errand Controller](#errand-controller)
         1. [Watches](#watches-in-errand-controller)
         2. [Reconciliation](#reconciliation-in-errand-controller)
         3. [Highlights](#highlights-in-errand-controller)
      2. [Job Controller](#job-controller)
         1. [Watches](#watches-in-job-controller)
         2. [Reconciliation](#reconciliation-in-job-controller)
         3. [Highlights](#highlights-in-job-controller)
   3. [Relationship with the BDPL component](#relationship-with-the-bdpl-component)
   4. [ExtendedJob Examples](#extendedjob-examples)

## Description

An `ExtendedJob` allows the developer to run jobs when something interesting happens. It also allows the developer to store the output to a file /mnt/quarks/output.json which is transformed into a `Secret` later.
The job started by an `ExtendedJob` is deleted automatically after it succeeds.

There are two different kinds of `ExtendedJob`:

- **one-offs**: automatically runs once after it's created
- **errands**: needs to be run manually by a user

## ExtendedJob Component

The **ExtendedJob** component is a categorization of a set of controllers, under the same group. Inside the **ExtendedJob** component we have a set of 2 controllers together with one separate reconciliation loop per controller.

Figure 1, illustrates the **ExtendedJob** component diagram and the set of controllers it uses.

![flow-extendeds-job](quarks_ejobcomponent_flow.png)
*Fig. 1: The ExtendedJob component*

### **_Errand Controller_**

![errand-controller-flow](quarks_ejoberrandcontroller_flow.png)
*Fig. 2: The Errand controller flow*

This is the controller responsible for implementing **Errands**, this will lead to the generation of a Kubernetes job, in order to complete a task.

#### Watches in Errand controller

- `ExtendedJob` resources: Create and Update
- `ConfigMaps`: Update
- `Secrets`: Create and Update

#### Reconciliation in Errand controller

- When an `ExtendedJob` instance is generated, it will create an associated Kubernetes Job.
- The generation of new Kubernetes Jobs also serves as the trigger for the `Job Controller`, to start the Reconciliation.

#### Highlights in Errand controller

##### Errands

- Errands are run manually by the user. They are created by setting `trigger.strategy: manual`.

- After the `ExtendedJob` is created, run an errand by editing and applying the
manifest, i.e. via `kubectl edit errand1` and change `trigger.strategy: manual` to `trigger.strategy: now`. A `kubectl patch` is also a good way to trigger this type of `ExtendedJob`. After completion, this value is reset to `manual`.

##### Auto-Errands

- One-off jobs run directly when created, just like native k8s jobs.

- They are created with `trigger.strategy: once` and switch to `done` when
finished.

- If a versioned secret is referenced in the pod spec of an `ExtendedJob`, the most recent
version of that secret will be used when the batchv1.Job is created.

##### Restarting on Config Change

- Just like an `ExtendedStatefulSet`, a **one-off** `ExtendedJob` can
automatically be restarted if its environment/mounts have changed, due to a
`configMap` or a `secret` being updated. This also works for [versioned secrets](#versioned-secrets). This requires the attribute `updateOnConfigChange` to be set to true.

- Once `updateOnConfigChange` is enabled, modifying the `data` of any `ConfigMap` or `Secret` referenced by the `template` section of the job will trigger the job again.

##### Persisted Output

- The developer can specify a `Secret` where the standard output/error output of the `ExtendedJob` is stored.

- One secret is created or overwritten per container in the pod. The secrets' names are `<namePrefix>-<containerName>`.

- The only supported output type currently is JSON with a flat structure, i.e. all values being string values.

- The developer should ensure that he/she redirects the JSON output to a file named output.json in /mnt/quarks volume mount at the end of the container script. An example of the command field in the extendedjob spec will look like this 

```
command: ["/bin/sh"]
args: ["-c","json='{\"foo\": \"1\", \"bar\": \"baz\"}' && echo $json >> /mnt/quarks/output.json"]
```

- The secret is created by a side container in extendedjob pod which captures the create event of /mnt/quarks/output.json file.

- Secrets are created for each container in the extended job pod spec.

- **Note:** Output of previous runs is overwritten.

- The behavior of storing the output is controlled by specifying the following parameters:
  - `namePrefix` - Prefix for the name of the secret(s) that will hold the output.
  - `outputType` - Currently only `json` is supported. (default: `json`)
  - `secretLabels` - An optional map of labels which will be attached to the generated secret(s)
  - `writeOnFailure` - if true, output is written even though the Job failed. (default: `false`)
  - `versioned` - if true, the output is written in a [Versioned Secret](#versioned-secrets)

##### Versioned Secrets

Versioned Secrets are a set of `Secrets`, where each of them is immutable, and contains data for one iteration. Implementation can be found in the [versionedsecretstore](https://github.com/cloudfoundry-incubator/cf-operator/blob/master/pkg/kube/util/versionedsecretstore) package.

When an `ExtendedJob` is configured to save to "Versioned Secrets", the controller looks for the `Secret` with the largest ordinal, adds `1` to that value, and _creates a new Secret_.

Each versioned secret has the following characteristics:

- its name is calculated like this: `<name>-v<ORDINAL>` e.g. `mysecret-v2`
- it has the following labels:
  - `fissile.cloudfoundry.org/secret-kind` with a value of `versionedSecret`
  - `fissile.cloudfoundry.org/secret-version` with a value set to the `ordinal` of the secret
- an annotation of `fissile.cloudfoundry.org/source-description` that contains arbitrary information about the creator of the secret

### **_Job Controller_**

![job-controller-flow](quarks_ejobjobcontroller_flow.png)
*Fig. 3: The Job controller flow*

This is an auxiliary controller that relies on the Errand Controller output. It will be watching for Kubernetes Jobs that have Succeeded and deletes the job. If the jobpod of the succeeded job has a label `delete=pod`, it deletes the job pod too.

#### Watches in job controller

- `Jobs`: Succeeded 

#### Reconciliation in job controller

- Deletes succeeded job and its pod.

## Relationship with the BDPL component

![bdpl-ejob-relationship](quarks_bdpl_and_ejob_flow.png)
*Fig. 4: Relationship between the BDPL controller and the ExtendedJob component*

Figure 4 illustrates the interaction of the **BOSHDeployment** Controller with the **Errand** Controller and how the output of this one serves as the trigger for the **Job** Controller.

## `ExtendedJob` Examples

See https://github.com/cloudfoundry-incubator/cf-operator/tree/master/docs/examples/extended-job
