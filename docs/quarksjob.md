# QuarksJob

1. [QuarksJob](#quarksjob)
   1. [QuarksJob Component](#quarksjob-component)
      1. [Errand Controller](#errand-controller)
         1. [Watches](#watches-in-errand-controller)
         2. [Reconciliation](#reconciliation-in-errand-controller)
         3. [Highlights](#highlights-in-errand-controller)
      2. [Job Controller](#job-controller)
         1. [Watches](#watches-in-job-controller)
         2. [Reconciliation](#reconciliation-in-job-controller)
         3. [Highlights](#highlights-in-job-controller)
   2. [Relationship with the BDPL component](#relationship-with-the-bdpl-component)
   3. [QuarksJob Examples](#quarksjob-examples)

## Description


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
