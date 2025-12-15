# MongoDB::Atlas::StreamProcessor

## Impact
The following components use this resource and are potentially impacted by any changes. They should also be validated to ensure the changes do not cause a regression.
 - Stream Processor L1 CDK constructor


## Prerequisites
### Resources needed to run the manual QA
All resources are created as part of `cfn-testing-helper.sh`:

- Atlas Project
- Atlas Stream Instance/Workspace (LONG-RUNNING operation, can take 10-30+ minutes)
- Cluster (for DLQ connection testing - inputs_3)
- Stream Connection (for DLQ connection testing - inputs_3)

**IMPORTANT**: Stream Instance/Workspace creation is a LONG-RUNNING operation that can take 10-30+ minutes. The `cfn-test-create-inputs.sh` script will create the workspace and wait for it to be ready before proceeding.

## Manual QA
Please follow the steps in [TESTING.md](../../../TESTING.md).


### Success criteria when testing the resource
1. A Stream Processor should be created in the specified test project for the specified Atlas Stream workspace/instance:
   - Verify the processor appears in the Atlas UI under the Stream Processing section
   - Verify the processor name matches the `ProcessorName` in the template
   - Verify the pipeline configuration matches the `Pipeline` in the template
   - Verify the state matches the `State` in the template (CREATED, STARTED, or STOPPED)

2. For processors with DLQ configuration (inputs_3):
   - Verify the DLQ options are correctly configured
   - Verify the connection name, database, and collection are set correctly

3. Test backward compatibility:
   - Test with `WorkspaceName` (preferred field)
   - Test with `InstanceName` (deprecated field, should still work)

4. Test state transitions:
   - Create with `State: CREATED` and verify processor is in CREATED state
   - Create with `State: STARTED` and verify processor transitions to STARTED state
   - Update state from CREATED to STARTED and verify transition
   - Update state from STARTED to STOPPED and verify transition

5. Test timeout and cleanup behavior:
   - Verify `Timeouts.Create` is respected
   - Verify `DeleteOnCreateTimeout` behavior when timeout occurs

6. Ensure general [CFN resource success criteria](../../../TESTING.md#success-criteria-when-testing-the-resource) for this resource is met.


## Important Links
- [API Documentation](https://www.mongodb.com/docs/api/doc/atlas-admin-api-v2/group/endpoint-streams)
- [Resource Usage Documentation](https://www.mongodb.com/docs/atlas/atlas-sp/overview/)

## Unit Testing Locally

The local tests are integrated with the AWS `sam local` and `cfn invoke` tooling features:

```
sam local start-lambda --skip-pull-image
```
then in another shell:
```bash
repo_root=$(git rev-parse --show-toplevel)
cd ${repo_root}/cfn-resources/stream-processor
cfn invoke resource CREATE stream-processor-sample-cfn-request.json
cfn invoke resource DELETE stream-processor-sample-cfn-request.json
cd -
```

Both CREATE & DELETE tests must pass.

## Test Input Files

The test directory contains the following input files:

- `inputs_1_create.json` / `inputs_1_update.json`: Basic stream processor with WorkspaceName, CREATED state
- `inputs_2_create.json` / `inputs_2_update.json`: Stream processor with STARTED state, timeout configuration, and DeleteOnCreateTimeout
- `inputs_3_create.json` / `inputs_3_update.json`: Stream processor with InstanceName (backward compatibility) and DLQ options

All input files respect:
- AWS-only behavior (no Azure/GCP-only parameters)
- Required fields: ProjectId, ProcessorName, Pipeline
- Backward compatibility: Supports both WorkspaceName and InstanceName
- Schema validation: All fields match the final CFN schema
