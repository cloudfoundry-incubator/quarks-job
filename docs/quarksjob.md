# QuarksJob

1. [QuarksJob](#quarksjob)
   1. [Description](#description)
   2. [QuarksJob Component](#quarksjob-component)
      1. [Errand Controller](#errand-controller)
         1. [Watches](#watches-in-errand-controller)
         2. [Reconciliation](#reconciliation-in-errand-controller)
         3. [Highlights](#highlights-in-errand-controller)
      2. [Job Controller](#job-controller)
         1. [Watches](#watches-in-job-controller)
         2. [Reconciliation](#reconciliation-in-job-controller)
         3. [Highlights](#highlights-in-job-controller)
   3. [Relationship with the BDPL component](#relationship-with-the-bdpl-component)
   4. [QuarksJob Examples](#quarksjob-examples)

## Description

An `QuarksJob` allows the developer to run jobs when something interesting happens. It also allows the developer to store the output to a file /mnt/quarks/output.json which is transformed into a `Secret` later.
The job started by an `QuarksJob` is deleted automatically after it succeeds.

There are two different kinds of `QuarksJob`:

- **one-offs**: automatically runs once after it's created
- **errands**: needs to be run manually by a user

## QuarksJob Component

The **QuarksJob** component is a categorization of a set of controllers, under the same group. Inside the **QuarksJob** component we have a set of 2 controllers together with one separate reconciliation loop per controller.

Figure 1, illustrates the **QuarksJob** component diagram and the set of controllers it uses.

![flow-quarks-job](quarks_qjobcomponent_flow.png)
*Fig. 1: The QuarksJob component*

### **_Errand Controller_**

![errand-controller-flow](quarks_qjoberrandcontroller_flow.png)
*Fig. 2: The Errand controller flow*

This is the controller responsible for implementing **Errands**, this will lead to the generation of a Kubernetes job, in order to complete a task.

#### Watches in Errand controller

- `QuarksJob` resources: Create and Update
- `ConfigMaps`: Update
- `Secrets`: Create and Update

#### Reconciliation in Errand controller

- When an `QuarksJob` instance is generated, it will create an associated Kubernetes Job.
- The generation of new Kubernetes Jobs also serves as the trigger for the `Job Controller`, to start the Reconciliation.

#### Highlights in Errand controller

##### Errands

- Errands are run manually by the user. They are created by setting `trigger.strategy: manual`.

- After the `QuarksJob` is created, run an errand by editing and applying the
manifest, i.e. via `kubectl edit errand1` and change `trigger.strategy: manual` to `trigger.strategy: now`. A `kubectl patch` is also a good way to trigger this type of `QuarksJob`. After completion, this value is reset to `manual`.

##### Auto-Errands

- One-off jobs run directly when created, just like native k8s jobs.

- They are created with `trigger.strategy: once` and switch to `done` when
finished.

- If a versioned secret is referenced in the pod spec of an `QuarksJob`, the most recent
version of that secret will be used when the batchv1.Job is created.

##### Restarting on Config Change

- A **one-off** `QuarksJob` can
automatically be restarted if its environment/mounts have changed, due to a
`configMap` or a `secret` being updated. This also works for [versioned secrets](#versioned-secrets). This requires the attribute `updateOnConfigChange` to be set to true.

- Once `updateOnConfigChange` is enabled, modifying the `data` of any `ConfigMap` or `Secret` referenced by the `template` section of the job will trigger the job again.

##### Versioned Secrets

Versioned Secrets are a set of `Secrets`, where each of them is immutable, and contains data for one iteration. Implementation can be found in the [versionedsecretstore](https://github.com/cloudfoundry-incubator/quarks-utils/tree/master/pkg/versionedsecretstore) package.

When an `QuarksJob` is configured to save to "Versioned Secrets", the controller looks for the `Secret` with the largest ordinal, adds `1` to that value, and _creates a new Secret_.

Each versioned secret has the following characteristics:

- its name is calculated like this: `<name>-v<ORDINAL>` e.g. `mysecret-v2`
- it has the following labels:
  - `quarks.cloudfoundry.org/secret-kind` with a value of `versionedSecret`
  - `quarks.cloudfoundry.org/secret-version` with a value set to the `ordinal` of the secret
- an annotation of `quarks.cloudfoundry.org/source-description` that contains arbitrary information about the creator of the secret

### **_Job Controller_**

![job-controller-flow](quarks_qjobjobcontroller_flow.png)
*Fig. 3: The Job controller flow*

This is an auxiliary controller that relies on the Errand Controller output. It will be watching for Kubernetes Jobs that have Succeeded and deletes the job. If the jobpod of the succeeded job has a label `delete=pod`, it deletes the job pod too.

#### Watches in job controller

- `Jobs`: Succeeded

#### Reconciliation in job controller

- Deletes succeeded job and its pod.

## Relationship with the BDPL component

![bdpl-qjob-relationship](quarks_bdpl_and_qjob_flow.png)
*Fig. 4: Relationship between the BDPL controller and the QuarksJob component*

Figure 4 illustrates the interaction of the **BOSHDeployment** Controller with the **Errand** Controller and how the output of this one serves as the trigger for the **Job** Controller.

## `QuarksJob` Examples

See https://github.com/cloudfoundry-incubator/quarks-job/tree/master/docs/examples
