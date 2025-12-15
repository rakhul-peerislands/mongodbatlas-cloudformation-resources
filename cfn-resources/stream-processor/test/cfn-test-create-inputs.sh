#!/usr/bin/env bash
# cfn-test-create-inputs.sh
#
# This tool generates json files in the inputs/ for `cfn test`.
#

set -euo pipefail

rm -rf inputs
mkdir inputs

#set profile
profile="default"
if [ ${MONGODB_ATLAS_PROFILE+x} ]; then
	echo "profile set to ${MONGODB_ATLAS_PROFILE}"
	profile=${MONGODB_ATLAS_PROFILE}
fi

projectName="${1:-$PROJECT_NAME}"
echo "$projectName"
projectId=$(atlas projects list --output json | jq --arg NAME "${projectName}" -r '.results[] | select(.name==$NAME) | .id')
if [ -z "$projectId" ]; then
	projectId=$(atlas projects create "${projectName}" --output=json | jq -r '.id')

	echo -e "Created project \"${projectName}\" with id: ${projectId}\n"
else
	echo -e "FOUND project \"${projectName}\" with id: ${projectId}\n"
fi
echo -e "=====\nrun this command to clean up\n=====\nmongocli iam projects delete ${projectId} --force\n====="

# Create Stream Instance/Workspace (this is a LONG-RUNNING operation, can take 10-30+ minutes)
workspaceName="stream-workspace-$(date +%s)-$RANDOM"
cloudProvider="AWS"

echo -e "Creating Stream Instance/Workspace \"${workspaceName}\" (this may take 10-30+ minutes)...\n"
atlas streams instances create "${workspaceName}" --projectId "${projectId}" --region VIRGINIA_USA --provider ${cloudProvider}
echo -e "Waiting for Stream Instance/Workspace \"${workspaceName}\" to be ready...\n"
atlas streams instances watch "${workspaceName}" --projectId "${projectId}"
echo -e "Stream Instance/Workspace \"${workspaceName}\" is ready\n"

# For inputs_3 (DLQ testing), we need a cluster and stream connection
# Create cluster for DLQ connection (if needed)
clusterName="cluster-$(date +%s)-$RANDOM"
connectionName="stream-connection-$(date +%s)-$RANDOM"

echo -e "Creating Cluster \"${clusterName}\" for DLQ connection...\n"
atlas clusters create "${clusterName}" --projectId "${projectId}" --backup --provider AWS --region US_EAST_1 --members 3 --tier M10 --diskSizeGB 10 --output=json
atlas clusters watch "${clusterName}" --projectId "${projectId}"
echo -e "Created Cluster \"${clusterName}\"\n"

echo -e "Creating Stream Connection \"${connectionName}\" for DLQ...\n"
atlas streams connections create "${connectionName}" \
	--projectId "${projectId}" \
	--instance "${workspaceName}" \
	--type Cluster \
	--clusterName "${clusterName}" \
	--dbRoleToExecute '{"role":"atlasAdmin","type":"BUILT_IN"}' \
	--output=json
echo -e "Created Stream Connection \"${connectionName}\"\n"

# Generate input files
jq --arg workspace_name "$workspaceName" \
	--arg project_id "$projectId" \
	--arg profile "$profile" \
	'.Profile?|=$profile
   | .ProjectId?|=$project_id
   | .WorkspaceName?|=$workspace_name' \
	"$(dirname "$0")/inputs_1_create.json" >"inputs/inputs_1_create.json"

jq --arg workspace_name "$workspaceName" \
	--arg project_id "$projectId" \
	--arg profile "$profile" \
	'.Profile?|=$profile
   | .ProjectId?|=$project_id
   | .WorkspaceName?|=$workspace_name' \
	"$(dirname "$0")/inputs_1_update.json" >"inputs/inputs_1_update.json"

jq --arg workspace_name "$workspaceName" \
	--arg project_id "$projectId" \
	--arg profile "$profile" \
	'.Profile?|=$profile
   | .ProjectId?|=$project_id
   | .WorkspaceName?|=$workspace_name' \
	"$(dirname "$0")/inputs_2_create.json" >"inputs/inputs_2_create.json"

jq --arg workspace_name "$workspaceName" \
	--arg project_id "$projectId" \
	--arg profile "$profile" \
	'.Profile?|=$profile
   | .ProjectId?|=$project_id
   | .WorkspaceName?|=$workspace_name' \
	"$(dirname "$0")/inputs_2_update.json" >"inputs/inputs_2_update.json"

jq --arg workspace_name "$workspaceName" \
	--arg project_id "$projectId" \
	--arg profile "$profile" \
	--arg connection_name "$connectionName" \
	--arg cluster_name "$clusterName" \
	'.Profile?|=$profile
   | .ProjectId?|=$project_id
   | .InstanceName?|=$workspace_name
   | .Options.Dlq.ConnectionName?|=$connection_name
   | .ClusterName?|=$cluster_name' \
	"$(dirname "$0")/inputs_3_create.json" >"inputs/inputs_3_create.json"

jq --arg workspace_name "$workspaceName" \
	--arg project_id "$projectId" \
	--arg profile "$profile" \
	--arg connection_name "$connectionName" \
	--arg cluster_name "$clusterName" \
	'.Profile?|=$profile
   | .ProjectId?|=$project_id
   | .InstanceName?|=$workspace_name
   | .Options.Dlq.ConnectionName?|=$connection_name
   | .ClusterName?|=$cluster_name' \
	"$(dirname "$0")/inputs_3_update.json" >"inputs/inputs_3_update.json"

echo -e "Test input files generated successfully in inputs/ directory\n"
