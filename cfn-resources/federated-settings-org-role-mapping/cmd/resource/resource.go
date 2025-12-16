// Copyright 2023 MongoDB Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resource

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	admin20250312010 "go.mongodb.org/atlas-sdk/v20250312010/admin"

	"github.com/aws-cloudformation/cloudformation-cli-go-plugin/cfn/handler"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	"github.com/mongodb/mongodbatlas-cloudformation-resources/util"
	"github.com/mongodb/mongodbatlas-cloudformation-resources/util/constants"
	"github.com/mongodb/mongodbatlas-cloudformation-resources/util/progressevent"
	"github.com/mongodb/mongodbatlas-cloudformation-resources/util/validator"
)

var CreateRequiredFields = []string{constants.FederationSettingsID, constants.OrgID, constants.ExternalGroupName, constants.RoleAssignments}
var ReadRequiredFields = []string{constants.FederationSettingsID, constants.ID, constants.OrgID}
var UpdateRequiredFields = []string{constants.FederationSettingsID, constants.OrgID, constants.ID, constants.ExternalGroupName, constants.RoleAssignments}
var DeleteRequiredFields = []string{constants.FederationSettingsID, constants.OrgID, constants.ID}
var ListRequiredFields = []string{constants.FederationSettingsID, constants.OrgID}

func validateModel(fields []string, model *Model) *handler.ProgressEvent {
	return validator.ValidateModel(fields, model)
}

func setup() {
	util.SetupLogger("mongodb-atlas-FederatedSettingsOrgRoleMapping")
}

// initEnvWithLatestClient is a variable that can be reassigned in tests for mocking
var initEnvWithLatestClient = func(req handler.Request, currentModel *Model, requiredFields []string) (*admin20250312010.APIClient, *handler.ProgressEvent) {
	setup()

	util.SetDefaultProfileIfNotDefined(&currentModel.Profile)

	if errEvent := validateModel(requiredFields, currentModel); errEvent != nil {
		return nil, errEvent
	}

	client, pe := util.NewAtlasClient(&req, currentModel.Profile)
	if pe != nil {
		return nil, pe
	}

	return client.AtlasSDK, nil
}

func Create(req handler.Request, prevModel *Model, currentModel *Model) (handler.ProgressEvent, error) {
	conn, peErr := initEnvWithLatestClient(req, currentModel, CreateRequiredFields)
	if peErr != nil {
		return *peErr, nil
	}

	federationSettingsID := currentModel.FederationSettingsId
	orgID := currentModel.OrgId

	requestBody, _, _ := NewRoleMappingRequest(currentModel)
	federatedSettingsOrganizationRoleMapping, resp, err := conn.FederatedAuthenticationApi.CreateRoleMapping(context.Background(), *federationSettingsID, *orgID, requestBody).Execute()
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusBadRequest && strings.Contains(err.Error(), "DUPLICATE_ROLE_MAPPING") {
			return progressevent.GetFailedEventByCode("Resource already exists",
				string(types.HandlerErrorCodeAlreadyExists)), nil
		}
		return progressevent.GetFailedEventByResponse(fmt.Sprintf("Error creating resource : %s", err.Error()),
			resp), nil
	}
	// GetRoleMappingModel will set the Id from the API response (matching Terraform's role_mapping_id behavior)
	return handler.ProgressEvent{
		OperationStatus: handler.Success,
		ResourceModel:   GetRoleMappingModel(federatedSettingsOrganizationRoleMapping, currentModel),
	}, nil
}

func Read(req handler.Request, prevModel *Model, currentModel *Model) (handler.ProgressEvent, error) {
	conn, peErr := initEnvWithLatestClient(req, currentModel, ReadRequiredFields)
	if peErr != nil {
		return *peErr, nil
	}

	federationSettingsID := currentModel.FederationSettingsId
	orgID := currentModel.OrgId
	roleMappingID := currentModel.Id

	federatedSettingsOrganizationRoleMapping, resp, err := conn.FederatedAuthenticationApi.
		GetRoleMapping(context.Background(), *federationSettingsID, *roleMappingID, *orgID).
		Execute()
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return progressevent.GetFailedEventByCode("Resource not found",
				string(types.HandlerErrorCodeNotFound)), nil
		}
		return progressevent.GetFailedEventByResponse(fmt.Sprintf("Error getting resource : %s", err.Error()),
			resp), nil
	}

	return handler.ProgressEvent{
		OperationStatus: handler.Success,
		ResourceModel:   GetRoleMappingModel(federatedSettingsOrganizationRoleMapping, currentModel),
	}, nil
}

func Update(req handler.Request, prevModel *Model, currentModel *Model) (handler.ProgressEvent, error) {
	conn, peErr := initEnvWithLatestClient(req, currentModel, UpdateRequiredFields)
	if peErr != nil {
		return *peErr, nil
	}

	federationSettingsID := currentModel.FederationSettingsId
	orgID := currentModel.OrgId
	roleMappingID := currentModel.Id

	// Get current resource state first (matching Terraform behavior)
	federatedSettingsOrganizationRoleMappingUpdate, _, err := conn.FederatedAuthenticationApi.
		GetRoleMapping(context.Background(), *federationSettingsID, *roleMappingID, *orgID).
		Execute()
	if err != nil {
		// Match Terraform behavior: return error directly without checking for 404
		return progressevent.GetFailedEventByResponse(fmt.Sprintf("error retrieving federation settings connected organization (%s): %s", *federationSettingsID, err.Error()),
			nil), nil
	}

	// Only update fields that have changed (matching Terraform's HasChange behavior)
	if hasExternalGroupNameChanged(prevModel, currentModel) && currentModel.ExternalGroupName != nil {
		federatedSettingsOrganizationRoleMappingUpdate.ExternalGroupName = *currentModel.ExternalGroupName
	}

	// Check if RoleAssignments changed
	roleAssignmentsChanged := prevModel == nil ||
		!roleAssignmentsEqual(prevModel.RoleAssignments, currentModel.RoleAssignments)

	if roleAssignmentsChanged {
		// Always update RoleAssignments when changed (matching Terraform's HasChange behavior)
		// RoleAssignments is required, so it should never be empty, but we handle it for safety
		roleAssignments := expandRoleAssignments(currentModel.RoleAssignments)
		federatedSettingsOrganizationRoleMappingUpdate.RoleAssignments = &roleAssignments
	}

	// Call update API
	updatedRoleMapping, _, err := conn.FederatedAuthenticationApi.
		UpdateRoleMapping(context.Background(), *federationSettingsID, *roleMappingID, *orgID, federatedSettingsOrganizationRoleMappingUpdate).
		Execute()
	if err != nil {
		// Match Terraform error message format
		return progressevent.GetFailedEventByResponse(fmt.Sprintf("error updating federation settings connected organization (%s): %s", *federationSettingsID, err.Error()),
			nil), nil
	}

	return handler.ProgressEvent{
		OperationStatus: handler.Success,
		Message:         "Update Complete",
		ResourceModel:   GetRoleMappingModel(updatedRoleMapping, currentModel),
	}, nil
}

func Delete(req handler.Request, prevModel *Model, currentModel *Model) (handler.ProgressEvent, error) {
	conn, peErr := initEnvWithLatestClient(req, currentModel, DeleteRequiredFields)
	if peErr != nil {
		return *peErr, nil
	}

	federationSettingsID := currentModel.FederationSettingsId
	orgID := currentModel.OrgId
	roleMappingID := currentModel.Id

	// Delete resource (matching Terraform behavior - returns error on failure including 404)
	resp, err := conn.FederatedAuthenticationApi.
		DeleteRoleMapping(context.Background(), *federationSettingsID, *roleMappingID, *orgID).
		Execute()
	if err != nil {
		return progressevent.GetFailedEventByResponse(fmt.Sprintf("Error deleting federated settings : %s", err.Error()),
			resp), nil
	}

	return handler.ProgressEvent{
		OperationStatus: handler.Success,
		Message:         "Delete Complete",
	}, nil
}

func List(req handler.Request, prevModel *Model, currentModel *Model) (handler.ProgressEvent, error) {
	conn, peErr := initEnvWithLatestClient(req, currentModel, ListRequiredFields)
	if peErr != nil {
		return *peErr, nil
	}

	federationSettingsID := currentModel.FederationSettingsId
	orgID := currentModel.OrgId

	federatedSettingsOrganizationRoleMappings, resp, err := conn.
		FederatedAuthenticationApi.
		ListRoleMappings(context.Background(), *federationSettingsID, *orgID).
		Execute()
	if err != nil {
		return progressevent.GetFailedEventByResponse(fmt.Sprintf("Error getting federated settings : %s", err.Error()),
			resp), nil
	}

	results := federatedSettingsOrganizationRoleMappings.GetResults()
	models := make([]any, 0, len(results))
	for i := range results {
		model := Model{
			Profile:              currentModel.Profile,
			OrgId:                currentModel.OrgId,
			FederationSettingsId: currentModel.FederationSettingsId,
			Id:                   results[i].Id,
			ExternalGroupName:    &results[i].ExternalGroupName,
		}
		if roleAssignments := results[i].GetRoleAssignments(); len(roleAssignments) > 0 {
			model.RoleAssignments = flattenRoleAssignments(roleAssignments)
		}
		models = append(models, model)
	}
	return handler.ProgressEvent{
		OperationStatus: handler.Success,
		Message:         "List Complete",
		ResourceModels:  models,
	}, nil
}
