// Copyright 2024 MongoDB Inc
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
	"fmt"
	"net/http"
	"testing"

	"github.com/aws-cloudformation/cloudformation-cli-go-plugin/cfn/handler"
	"github.com/mongodb/mongodbatlas-cloudformation-resources/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	admin20250312010 "go.mongodb.org/atlas-sdk/v20250312010/admin"
	"go.mongodb.org/atlas-sdk/v20250312010/mockadmin"
)

// Helper function to create a test model
func createTestModel() *Model {
	federationSettingsID := "507f1f77bcf86cd799439011"
	orgID := "507f1f77bcf86cd799439012"
	externalGroupName := "test-group"
	role := "ORG_MEMBER"
	groupId := "507f1f77bcf86cd799439013"

	return &Model{
		Profile:              util.StringPtr("default"),
		FederationSettingsId: &federationSettingsID,
		OrgId:                &orgID,
		ExternalGroupName:    &externalGroupName,
		RoleAssignments: []RoleAssignment{
			{
				Role:    &role,
				OrgId:   &orgID,
				GroupId: &groupId,
			},
		},
	}
}

// Helper function to create a test role mapping response
func createTestRoleMapping() *admin20250312010.AuthFederationRoleMapping {
	id := "507f1f77bcf86cd799439014"
	externalGroupName := "test-group"
	role := "ORG_MEMBER"
	orgID := "507f1f77bcf86cd799439012"
	groupId := "507f1f77bcf86cd799439013"

	roleAssignments := []admin20250312010.ConnectedOrgConfigRoleAssignment{
		{
			Role:    &role,
			OrgId:   &orgID,
			GroupId: &groupId,
		},
	}
	return &admin20250312010.AuthFederationRoleMapping{
		Id:                &id,
		ExternalGroupName: externalGroupName,
		RoleAssignments:   &roleAssignments,
	}
}

// Test validation errors
func TestCreateValidationErrors(t *testing.T) {
	testCases := map[string]struct {
		currentModel   *Model
		expectedStatus handler.Status
		expectedMsg    string
	}{
		"missingFederationSettingsId": {
			currentModel: &Model{
				OrgId:             util.StringPtr("507f1f77bcf86cd799439012"),
				ExternalGroupName: util.StringPtr("test-group"),
			},
			expectedStatus: handler.Failed,
			expectedMsg:    "required",
		},
		"missingOrgId": {
			currentModel: &Model{
				FederationSettingsId: util.StringPtr("507f1f77bcf86cd799439011"),
				ExternalGroupName:    util.StringPtr("test-group"),
			},
			expectedStatus: handler.Failed,
			expectedMsg:    "required",
		},
		"missingExternalGroupName": {
			currentModel: &Model{
				FederationSettingsId: util.StringPtr("507f1f77bcf86cd799439011"),
				OrgId:                util.StringPtr("507f1f77bcf86cd799439012"),
			},
			expectedStatus: handler.Failed,
			expectedMsg:    "required",
		},
		"missingRoleAssignments": {
			currentModel: &Model{
				FederationSettingsId: util.StringPtr("507f1f77bcf86cd799439011"),
				OrgId:                util.StringPtr("507f1f77bcf86cd799439012"),
				ExternalGroupName:    util.StringPtr("test-group"),
			},
			expectedStatus: handler.Failed,
			expectedMsg:    "required",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			req := handler.Request{
				RequestContext: handler.RequestContext{},
			}
			event, err := Create(req, nil, tc.currentModel)

			require.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, event.OperationStatus)
			assert.Contains(t, event.Message, tc.expectedMsg)
		})
	}
}

func TestReadValidationErrors(t *testing.T) {
	testCases := map[string]struct {
		currentModel   *Model
		expectedStatus handler.Status
		expectedMsg    string
	}{
		"missingFederationSettingsId": {
			currentModel: &Model{
				OrgId: util.StringPtr("507f1f77bcf86cd799439012"),
				Id:    util.StringPtr("507f1f77bcf86cd799439014"),
			},
			expectedStatus: handler.Failed,
			expectedMsg:    "required",
		},
		"missingOrgId": {
			currentModel: &Model{
				FederationSettingsId: util.StringPtr("507f1f77bcf86cd799439011"),
				Id:                   util.StringPtr("507f1f77bcf86cd799439014"),
			},
			expectedStatus: handler.Failed,
			expectedMsg:    "required",
		},
		"missingId": {
			currentModel: &Model{
				FederationSettingsId: util.StringPtr("507f1f77bcf86cd799439011"),
				OrgId:                util.StringPtr("507f1f77bcf86cd799439012"),
			},
			expectedStatus: handler.Failed,
			expectedMsg:    "required",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			req := handler.Request{
				RequestContext: handler.RequestContext{},
			}
			event, err := Read(req, nil, tc.currentModel)

			require.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, event.OperationStatus)
			assert.Contains(t, event.Message, tc.expectedMsg)
		})
	}
}

func TestUpdateValidationErrors(t *testing.T) {
	testCases := map[string]struct {
		currentModel   *Model
		expectedStatus handler.Status
		expectedMsg    string
	}{
		"missingFederationSettingsId": {
			currentModel: &Model{
				OrgId:             util.StringPtr("507f1f77bcf86cd799439012"),
				Id:                util.StringPtr("507f1f77bcf86cd799439014"),
				ExternalGroupName: util.StringPtr("test-group"),
			},
			expectedStatus: handler.Failed,
			expectedMsg:    "required",
		},
		"missingOrgId": {
			currentModel: &Model{
				FederationSettingsId: util.StringPtr("507f1f77bcf86cd799439011"),
				Id:                   util.StringPtr("507f1f77bcf86cd799439014"),
				ExternalGroupName:    util.StringPtr("test-group"),
			},
			expectedStatus: handler.Failed,
			expectedMsg:    "required",
		},
		"missingId": {
			currentModel: &Model{
				FederationSettingsId: util.StringPtr("507f1f77bcf86cd799439011"),
				OrgId:                util.StringPtr("507f1f77bcf86cd799439012"),
				ExternalGroupName:    util.StringPtr("test-group"),
			},
			expectedStatus: handler.Failed,
			expectedMsg:    "required",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			req := handler.Request{
				RequestContext: handler.RequestContext{},
			}
			event, err := Update(req, nil, tc.currentModel)

			require.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, event.OperationStatus)
			assert.Contains(t, event.Message, tc.expectedMsg)
		})
	}
}

func TestDeleteValidationErrors(t *testing.T) {
	testCases := map[string]struct {
		currentModel   *Model
		expectedStatus handler.Status
		expectedMsg    string
	}{
		"missingFederationSettingsId": {
			currentModel: &Model{
				OrgId: util.StringPtr("507f1f77bcf86cd799439012"),
			},
			expectedStatus: handler.Failed,
			expectedMsg:    "required",
		},
		"missingOrgId": {
			currentModel: &Model{
				FederationSettingsId: util.StringPtr("507f1f77bcf86cd799439011"),
			},
			expectedStatus: handler.Failed,
			expectedMsg:    "required",
		},
		"missingId": {
			currentModel: &Model{
				FederationSettingsId: util.StringPtr("507f1f77bcf86cd799439011"),
				OrgId:                util.StringPtr("507f1f77bcf86cd799439012"),
			},
			expectedStatus: handler.Failed,
			expectedMsg:    "required",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			req := handler.Request{
				RequestContext: handler.RequestContext{},
			}
			event, err := Delete(req, nil, tc.currentModel)

			require.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, event.OperationStatus)
			assert.Contains(t, event.Message, tc.expectedMsg)
		})
	}
}

func TestListValidationErrors(t *testing.T) {
	testCases := map[string]struct {
		currentModel   *Model
		expectedStatus handler.Status
		expectedMsg    string
	}{
		"missingFederationSettingsId": {
			currentModel: &Model{
				OrgId: util.StringPtr("507f1f77bcf86cd799439012"),
			},
			expectedStatus: handler.Failed,
			expectedMsg:    "required",
		},
		"missingOrgId": {
			currentModel: &Model{
				FederationSettingsId: util.StringPtr("507f1f77bcf86cd799439011"),
			},
			expectedStatus: handler.Failed,
			expectedMsg:    "required",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			req := handler.Request{
				RequestContext: handler.RequestContext{},
			}
			event, err := List(req, nil, tc.currentModel)

			require.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, event.OperationStatus)
			assert.Contains(t, event.Message, tc.expectedMsg)
		})
	}
}

// Test CRUD operations with mocks
func TestCreateWithMocks(t *testing.T) {
	// Save original function
	originalInitEnv := initEnvWithLatestClient
	defer func() {
		initEnvWithLatestClient = originalInitEnv
	}()

	testCases := map[string]struct {
		req            handler.Request
		currentModel   *Model
		mockSetup      func(*mockadmin.FederatedAuthenticationApi)
		expectedStatus handler.Status
	}{
		"successfulCreate": {
			req: handler.Request{
				RequestContext: handler.RequestContext{},
			},
			currentModel: createTestModel(),
			mockSetup: func(m *mockadmin.FederatedAuthenticationApi) {
				req := admin20250312010.CreateRoleMappingApiRequest{ApiService: m}
				m.EXPECT().CreateRoleMapping(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(req)
				roleMapping := createTestRoleMapping()
				m.EXPECT().CreateRoleMappingExecute(mock.Anything).Return(roleMapping, &http.Response{StatusCode: 200}, nil)
			},
			expectedStatus: handler.Success,
		},
		"duplicateRoleMapping": {
			req: handler.Request{
				RequestContext: handler.RequestContext{},
			},
			currentModel: createTestModel(),
			mockSetup: func(m *mockadmin.FederatedAuthenticationApi) {
				req := admin20250312010.CreateRoleMappingApiRequest{ApiService: m}
				m.EXPECT().CreateRoleMapping(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(req)
				m.EXPECT().CreateRoleMappingExecute(mock.Anything).Return(nil, &http.Response{StatusCode: 400}, fmt.Errorf("DUPLICATE_ROLE_MAPPING"))
			},
			expectedStatus: handler.Failed,
		},
		"apiError": {
			req: handler.Request{
				RequestContext: handler.RequestContext{},
			},
			currentModel: createTestModel(),
			mockSetup: func(m *mockadmin.FederatedAuthenticationApi) {
				req := admin20250312010.CreateRoleMappingApiRequest{ApiService: m}
				m.EXPECT().CreateRoleMapping(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(req)
				m.EXPECT().CreateRoleMappingExecute(mock.Anything).Return(nil, &http.Response{StatusCode: 500}, fmt.Errorf("API error"))
			},
			expectedStatus: handler.Failed,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			mockApi := mockadmin.NewFederatedAuthenticationApi(t)
			tc.mockSetup(mockApi)

			mockClient := &admin20250312010.APIClient{}
			mockClient.FederatedAuthenticationApi = mockApi

			initEnvWithLatestClient = func(req handler.Request, currentModel *Model, requiredFields []string) (*admin20250312010.APIClient, *handler.ProgressEvent) {
				return mockClient, nil
			}

			event, err := Create(tc.req, nil, tc.currentModel)

			require.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, event.OperationStatus)
		})
	}
}

func TestReadWithMocks(t *testing.T) {
	// Save original function
	originalInitEnv := initEnvWithLatestClient
	defer func() {
		initEnvWithLatestClient = originalInitEnv
	}()

	testCases := map[string]struct {
		req            handler.Request
		currentModel   *Model
		mockSetup      func(*mockadmin.FederatedAuthenticationApi)
		expectedStatus handler.Status
	}{
		"successfulRead": {
			req: handler.Request{
				RequestContext: handler.RequestContext{},
			},
			currentModel: func() *Model {
				m := createTestModel()
				id := "507f1f77bcf86cd799439014"
				m.Id = &id
				return m
			}(),
			mockSetup: func(m *mockadmin.FederatedAuthenticationApi) {
				req := admin20250312010.GetRoleMappingApiRequest{ApiService: m}
				m.EXPECT().GetRoleMapping(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(req)
				roleMapping := createTestRoleMapping()
				m.EXPECT().GetRoleMappingExecute(mock.Anything).Return(roleMapping, &http.Response{StatusCode: 200}, nil)
			},
			expectedStatus: handler.Success,
		},
		"readNotFound": {
			req: handler.Request{
				RequestContext: handler.RequestContext{},
			},
			currentModel: func() *Model {
				m := createTestModel()
				id := "507f1f77bcf86cd799439014"
				m.Id = &id
				return m
			}(),
			mockSetup: func(m *mockadmin.FederatedAuthenticationApi) {
				req := admin20250312010.GetRoleMappingApiRequest{ApiService: m}
				m.EXPECT().GetRoleMapping(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(req)
				m.EXPECT().GetRoleMappingExecute(mock.Anything).Return(nil, &http.Response{StatusCode: 404}, fmt.Errorf("not found"))
			},
			expectedStatus: handler.Failed,
		},
		"apiError": {
			req: handler.Request{
				RequestContext: handler.RequestContext{},
			},
			currentModel: func() *Model {
				m := createTestModel()
				id := "507f1f77bcf86cd799439014"
				m.Id = &id
				return m
			}(),
			mockSetup: func(m *mockadmin.FederatedAuthenticationApi) {
				req := admin20250312010.GetRoleMappingApiRequest{ApiService: m}
				m.EXPECT().GetRoleMapping(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(req)
				m.EXPECT().GetRoleMappingExecute(mock.Anything).Return(nil, &http.Response{StatusCode: 500}, fmt.Errorf("server error"))
			},
			expectedStatus: handler.Failed,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			mockApi := mockadmin.NewFederatedAuthenticationApi(t)
			tc.mockSetup(mockApi)

			mockClient := &admin20250312010.APIClient{}
			mockClient.FederatedAuthenticationApi = mockApi

			initEnvWithLatestClient = func(req handler.Request, currentModel *Model, requiredFields []string) (*admin20250312010.APIClient, *handler.ProgressEvent) {
				return mockClient, nil
			}

			event, err := Read(tc.req, nil, tc.currentModel)

			require.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, event.OperationStatus)
		})
	}
}

func TestUpdateWithMocks(t *testing.T) {
	// Save original function
	originalInitEnv := initEnvWithLatestClient
	defer func() {
		initEnvWithLatestClient = originalInitEnv
	}()

	testCases := map[string]struct {
		req            handler.Request
		prevModel      *Model
		currentModel   *Model
		mockSetup      func(*mockadmin.FederatedAuthenticationApi)
		expectedStatus handler.Status
	}{
		"successfulUpdate": {
			req: handler.Request{
				RequestContext: handler.RequestContext{},
			},
			prevModel: createTestModel(),
			currentModel: func() *Model {
				m := createTestModel()
				id := "507f1f77bcf86cd799439014"
				m.Id = &id
				newName := "updated-group"
				m.ExternalGroupName = &newName
				return m
			}(),
			mockSetup: func(m *mockadmin.FederatedAuthenticationApi) {
				// Get current state
				req1 := admin20250312010.GetRoleMappingApiRequest{ApiService: m}
				m.EXPECT().GetRoleMapping(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(req1).Once()
				roleMapping := createTestRoleMapping()
				m.EXPECT().GetRoleMappingExecute(mock.Anything).Return(roleMapping, &http.Response{StatusCode: 200}, nil).Once()

				// Update
				req2 := admin20250312010.UpdateRoleMappingApiRequest{ApiService: m}
				m.EXPECT().UpdateRoleMapping(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(req2)
				updatedRoleMapping := createTestRoleMapping()
				updatedRoleMapping.ExternalGroupName = "updated-group"
				m.EXPECT().UpdateRoleMappingExecute(mock.Anything).Return(updatedRoleMapping, &http.Response{StatusCode: 200}, nil)
			},
			expectedStatus: handler.Success,
		},
		"updateRoleAssignments": {
			req: handler.Request{
				RequestContext: handler.RequestContext{},
			},
			prevModel: createTestModel(),
			currentModel: func() *Model {
				m := createTestModel()
				id := "507f1f77bcf86cd799439014"
				m.Id = &id
				newRole := "ORG_OWNER"
				m.RoleAssignments = []RoleAssignment{
					{
						Role:    &newRole,
						OrgId:   m.OrgId,
						GroupId: m.RoleAssignments[0].GroupId,
					},
				}
				return m
			}(),
			mockSetup: func(m *mockadmin.FederatedAuthenticationApi) {
				// Get current state
				req1 := admin20250312010.GetRoleMappingApiRequest{ApiService: m}
				m.EXPECT().GetRoleMapping(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(req1).Once()
				roleMapping := createTestRoleMapping()
				m.EXPECT().GetRoleMappingExecute(mock.Anything).Return(roleMapping, &http.Response{StatusCode: 200}, nil).Once()

				// Update
				req2 := admin20250312010.UpdateRoleMappingApiRequest{ApiService: m}
				m.EXPECT().UpdateRoleMapping(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(req2)
				updatedRoleMapping := createTestRoleMapping()
				m.EXPECT().UpdateRoleMappingExecute(mock.Anything).Return(updatedRoleMapping, &http.Response{StatusCode: 200}, nil)
			},
			expectedStatus: handler.Success,
		},
		"updateNotFound": {
			req: handler.Request{
				RequestContext: handler.RequestContext{},
			},
			prevModel: createTestModel(),
			currentModel: func() *Model {
				m := createTestModel()
				id := "507f1f77bcf86cd799439014"
				m.Id = &id
				return m
			}(),
			mockSetup: func(m *mockadmin.FederatedAuthenticationApi) {
				req1 := admin20250312010.GetRoleMappingApiRequest{ApiService: m}
				m.EXPECT().GetRoleMapping(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(req1)
				m.EXPECT().GetRoleMappingExecute(mock.Anything).Return(nil, &http.Response{StatusCode: 404}, fmt.Errorf("not found"))
			},
			expectedStatus: handler.Failed,
		},
		"updateApiError": {
			req: handler.Request{
				RequestContext: handler.RequestContext{},
			},
			prevModel: createTestModel(),
			currentModel: func() *Model {
				m := createTestModel()
				id := "507f1f77bcf86cd799439014"
				m.Id = &id
				return m
			}(),
			mockSetup: func(m *mockadmin.FederatedAuthenticationApi) {
				req1 := admin20250312010.GetRoleMappingApiRequest{ApiService: m}
				m.EXPECT().GetRoleMapping(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(req1).Once()
				roleMapping := createTestRoleMapping()
				m.EXPECT().GetRoleMappingExecute(mock.Anything).Return(roleMapping, &http.Response{StatusCode: 200}, nil).Once()

				req2 := admin20250312010.UpdateRoleMappingApiRequest{ApiService: m}
				m.EXPECT().UpdateRoleMapping(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(req2)
				m.EXPECT().UpdateRoleMappingExecute(mock.Anything).Return(nil, &http.Response{StatusCode: 500}, fmt.Errorf("update failed"))
			},
			expectedStatus: handler.Failed,
		},
		"updateWithNoChanges": {
			req: handler.Request{
				RequestContext: handler.RequestContext{},
			},
			prevModel: createTestModel(),
			currentModel: func() *Model {
				m := createTestModel()
				id := "507f1f77bcf86cd799439014"
				m.Id = &id
				return m
			}(),
			mockSetup: func(m *mockadmin.FederatedAuthenticationApi) {
				// Get current state
				req1 := admin20250312010.GetRoleMappingApiRequest{ApiService: m}
				m.EXPECT().GetRoleMapping(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(req1).Once()
				roleMapping := createTestRoleMapping()
				m.EXPECT().GetRoleMappingExecute(mock.Anything).Return(roleMapping, &http.Response{StatusCode: 200}, nil).Once()

				// Update should still be called even if no changes detected
				req2 := admin20250312010.UpdateRoleMappingApiRequest{ApiService: m}
				m.EXPECT().UpdateRoleMapping(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(req2)
				m.EXPECT().UpdateRoleMappingExecute(mock.Anything).Return(roleMapping, &http.Response{StatusCode: 200}, nil)
			},
			expectedStatus: handler.Success,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			mockApi := mockadmin.NewFederatedAuthenticationApi(t)
			tc.mockSetup(mockApi)

			mockClient := &admin20250312010.APIClient{}
			mockClient.FederatedAuthenticationApi = mockApi

			initEnvWithLatestClient = func(req handler.Request, currentModel *Model, requiredFields []string) (*admin20250312010.APIClient, *handler.ProgressEvent) {
				return mockClient, nil
			}

			event, err := Update(tc.req, tc.prevModel, tc.currentModel)

			require.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, event.OperationStatus)
		})
	}
}

func TestDeleteWithMocks(t *testing.T) {
	// Save original function
	originalInitEnv := initEnvWithLatestClient
	defer func() {
		initEnvWithLatestClient = originalInitEnv
	}()

	testCases := map[string]struct {
		req            handler.Request
		currentModel   *Model
		mockSetup      func(*mockadmin.FederatedAuthenticationApi)
		expectedStatus handler.Status
	}{
		"successfulDelete": {
			req: handler.Request{
				RequestContext: handler.RequestContext{},
			},
			currentModel: func() *Model {
				m := createTestModel()
				id := "507f1f77bcf86cd799439014"
				m.Id = &id
				return m
			}(),
			mockSetup: func(m *mockadmin.FederatedAuthenticationApi) {
				req := admin20250312010.DeleteRoleMappingApiRequest{ApiService: m}
				m.EXPECT().DeleteRoleMapping(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(req)
				m.EXPECT().DeleteRoleMappingExecute(mock.Anything).Return(&http.Response{StatusCode: 200}, nil)
			},
			expectedStatus: handler.Success,
		},
		"deleteWithError": {
			req: handler.Request{
				RequestContext: handler.RequestContext{},
			},
			currentModel: func() *Model {
				m := createTestModel()
				id := "507f1f77bcf86cd799439014"
				m.Id = &id
				return m
			}(),
			mockSetup: func(m *mockadmin.FederatedAuthenticationApi) {
				req := admin20250312010.DeleteRoleMappingApiRequest{ApiService: m}
				m.EXPECT().DeleteRoleMapping(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(req)
				m.EXPECT().DeleteRoleMappingExecute(mock.Anything).Return(&http.Response{StatusCode: 500}, fmt.Errorf("delete failed"))
			},
			expectedStatus: handler.Failed,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			mockApi := mockadmin.NewFederatedAuthenticationApi(t)
			tc.mockSetup(mockApi)

			mockClient := &admin20250312010.APIClient{}
			mockClient.FederatedAuthenticationApi = mockApi

			initEnvWithLatestClient = func(req handler.Request, currentModel *Model, requiredFields []string) (*admin20250312010.APIClient, *handler.ProgressEvent) {
				return mockClient, nil
			}

			event, err := Delete(tc.req, nil, tc.currentModel)

			require.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, event.OperationStatus)
		})
	}
}

func TestListWithMocks(t *testing.T) {
	// Save original function
	originalInitEnv := initEnvWithLatestClient
	defer func() {
		initEnvWithLatestClient = originalInitEnv
	}()

	testCases := map[string]struct {
		req            handler.Request
		currentModel   *Model
		mockSetup      func(*mockadmin.FederatedAuthenticationApi)
		expectedStatus handler.Status
	}{
		"successfulList": {
			req: handler.Request{
				RequestContext: handler.RequestContext{},
			},
			currentModel: createTestModel(),
			mockSetup: func(m *mockadmin.FederatedAuthenticationApi) {
				req := admin20250312010.ListRoleMappingsApiRequest{ApiService: m}
				m.EXPECT().ListRoleMappings(mock.Anything, mock.Anything, mock.Anything).Return(req)
				roleMapping1 := createTestRoleMapping()
				id2 := "507f1f77bcf86cd799439015"
				roleMapping2 := &admin20250312010.AuthFederationRoleMapping{
					Id:                &id2,
					ExternalGroupName: "test-group-2",
					RoleAssignments:   roleMapping1.RoleAssignments,
				}
				results := []admin20250312010.AuthFederationRoleMapping{*roleMapping1, *roleMapping2}
				paginated := &admin20250312010.PaginatedRoleMapping{
					Results: &results,
				}
				m.EXPECT().ListRoleMappingsExecute(mock.Anything).Return(paginated, &http.Response{StatusCode: 200}, nil)
			},
			expectedStatus: handler.Success,
		},
		"listWithEmptyResults": {
			req: handler.Request{
				RequestContext: handler.RequestContext{},
			},
			currentModel: createTestModel(),
			mockSetup: func(m *mockadmin.FederatedAuthenticationApi) {
				req := admin20250312010.ListRoleMappingsApiRequest{ApiService: m}
				m.EXPECT().ListRoleMappings(mock.Anything, mock.Anything, mock.Anything).Return(req)
				results := []admin20250312010.AuthFederationRoleMapping{}
				paginated := &admin20250312010.PaginatedRoleMapping{
					Results: &results,
				}
				m.EXPECT().ListRoleMappingsExecute(mock.Anything).Return(paginated, &http.Response{StatusCode: 200}, nil)
			},
			expectedStatus: handler.Success,
		},
		"listWithRoleAssignments": {
			req: handler.Request{
				RequestContext: handler.RequestContext{},
			},
			currentModel: createTestModel(),
			mockSetup: func(m *mockadmin.FederatedAuthenticationApi) {
				req := admin20250312010.ListRoleMappingsApiRequest{ApiService: m}
				m.EXPECT().ListRoleMappings(mock.Anything, mock.Anything, mock.Anything).Return(req)
				roleMapping := createTestRoleMapping()
				results := []admin20250312010.AuthFederationRoleMapping{*roleMapping}
				paginated := &admin20250312010.PaginatedRoleMapping{
					Results: &results,
				}
				m.EXPECT().ListRoleMappingsExecute(mock.Anything).Return(paginated, &http.Response{StatusCode: 200}, nil)
			},
			expectedStatus: handler.Success,
		},
		"listApiError": {
			req: handler.Request{
				RequestContext: handler.RequestContext{},
			},
			currentModel: createTestModel(),
			mockSetup: func(m *mockadmin.FederatedAuthenticationApi) {
				req := admin20250312010.ListRoleMappingsApiRequest{ApiService: m}
				m.EXPECT().ListRoleMappings(mock.Anything, mock.Anything, mock.Anything).Return(req)
				m.EXPECT().ListRoleMappingsExecute(mock.Anything).Return(nil, &http.Response{StatusCode: 500}, fmt.Errorf("list failed"))
			},
			expectedStatus: handler.Failed,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			mockApi := mockadmin.NewFederatedAuthenticationApi(t)
			tc.mockSetup(mockApi)

			mockClient := &admin20250312010.APIClient{}
			mockClient.FederatedAuthenticationApi = mockApi

			initEnvWithLatestClient = func(req handler.Request, currentModel *Model, requiredFields []string) (*admin20250312010.APIClient, *handler.ProgressEvent) {
				return mockClient, nil
			}

			event, err := List(tc.req, nil, tc.currentModel)

			require.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, event.OperationStatus)
			if tc.expectedStatus == handler.Success && name != "listWithEmptyResults" {
				if event.ResourceModels != nil && name == "successfulList" {
					assert.Greater(t, len(event.ResourceModels), 0)
				}
			}
		})
	}
}
