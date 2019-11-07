## Use Cases

- [Use Cases](#use-cases)
  - [qjob_output.yaml](#qjoboutputyaml)
  - [qjob_errand.yaml](#qjoberrandyaml)
  - [qjob_auto-errand.yaml](#qjobauto-errandyaml)
  - [qjob_auto-errand-updating.yaml](#qjobauto-errand-updatingyaml)
  - [qjob_auto-errand-deletes-pod.yaml](#qjobauto-errand-deletes-podyaml)

### qjob_output.yaml

This creates a `Secret` from the /mnt/quarks/output.json file in the container volume mount /mnt/quarks.

### qjob_errand.yaml

This exemplifies an errand that needs ot be run manually by the user. This is done by changing the trigger value to `now`.

```shell
kubectl patch qjob \
    -n NAMESPACE manual-sleep \
    -p '{"spec": {"trigger":{"strategy":"now"}}}'
```

### qjob_auto-errand.yaml

This creates a `Job` that runs once, to completion.

### qjob_auto-errand-updating.yaml

This demonstrates the capability to re-run an automated errand when a `ConfigMap` or `Secret` changes.

When `qjob_auto-errand-updating_updated.yaml` is applied, a new `Job` is created.

### qjob_auto-errand-deletes-pod.yaml

This auto-errand will automatically cleanup the completed pod once the `Job` runs successfully.
