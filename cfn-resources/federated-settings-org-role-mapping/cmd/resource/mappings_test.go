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
	"sort"
	"testing"

	"github.com/aws-cloudformation/cloudformation-cli-go-plugin/cfn/handler"
	"github.com/mongodb/mongodbatlas-cloudformation-resources/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admin20250312010 "go.mongodb.org/atlas-sdk/v20250312010/admin"
)

// Helper function to create a test model
func createTestModelForMappings() *Model {
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
func createTestRoleMappingForMappings() *admin20250312010.AuthFederationRoleMapping {
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

func TestGetRoleMappingModel(t *testing.T) {
	t.Run("withCurrentModel", func(t *testing.T) {
		currentModel := createTestModelForMappings()
		roleMapping := createTestRoleMappingForMappings()

		result := GetRoleMappingModel(roleMapping, currentModel)

		// Should preserve fields from currentModel
		assert.Equal(t, currentModel.Profile, result.Profile)
		assert.Equal(t, currentModel.FederationSettingsId, result.FederationSettingsId)
		assert.Equal(t, currentModel.OrgId, result.OrgId)
		// Should update with values from roleMapping
		assert.Equal(t, roleMapping.Id, result.Id)
		assert.NotNil(t, result.ExternalGroupName)
		assert.Equal(t, roleMapping.ExternalGroupName, *result.ExternalGroupName)
		assert.NotNil(t, result.RoleAssignments)
		assert.Greater(t, len(result.RoleAssignments), 0)
	})

	t.Run("withNilCurrentModel", func(t *testing.T) {
		roleMapping := createTestRoleMappingForMappings()

		result := GetRoleMappingModel(roleMapping, nil)

		assert.NotNil(t, result)
		assert.Equal(t, roleMapping.Id, result.Id)
		assert.NotNil(t, result.ExternalGroupName)
		assert.Equal(t, roleMapping.ExternalGroupName, *result.ExternalGroupName)
		assert.NotNil(t, result.RoleAssignments)
	})

	t.Run("withEmptyRoleAssignments", func(t *testing.T) {
		roleMapping := &admin20250312010.AuthFederationRoleMapping{
			Id:                util.StringPtr("507f1f77bcf86cd799439014"),
			ExternalGroupName: "test-group",
			RoleAssignments:   &[]admin20250312010.ConnectedOrgConfigRoleAssignment{},
		}

		result := GetRoleMappingModel(roleMapping, nil)

		assert.NotNil(t, result)
		assert.Nil(t, result.RoleAssignments)
	})
}

func TestNewRoleMappingRequest(t *testing.T) {
	t.Run("completeModel", func(t *testing.T) {
		model := createTestModelForMappings()

		req, pe, err := NewRoleMappingRequest(model)

		require.NoError(t, err)
		require.Equal(t, handler.ProgressEvent{}, pe)
		require.NotNil(t, req)
		assert.Nil(t, req.Id) // Id is nil in createTestModelForMappings
		assert.NotEmpty(t, req.ExternalGroupName)
		assert.NotNil(t, req.RoleAssignments)
		if req.RoleAssignments != nil {
			assert.Greater(t, len(*req.RoleAssignments), 0)
		}
	})

	t.Run("modelWithId", func(t *testing.T) {
		model := createTestModelForMappings()
		id := "507f1f77bcf86cd799439014"
		model.Id = &id

		req, pe, err := NewRoleMappingRequest(model)

		require.NoError(t, err)
		require.Equal(t, handler.ProgressEvent{}, pe)
		require.NotNil(t, req)
		// Id should NOT be included in the request (matching Terraform behavior)
		assert.Nil(t, req.Id)
	})

	t.Run("minimalModel", func(t *testing.T) {
		model := &Model{
			ExternalGroupName: util.StringPtr("test"),
		}

		req, pe, err := NewRoleMappingRequest(model)

		require.NoError(t, err)
		require.Equal(t, handler.ProgressEvent{}, pe)
		require.NotNil(t, req)
		assert.Nil(t, req.Id)
		assert.NotEmpty(t, req.ExternalGroupName)
		assert.Nil(t, req.RoleAssignments)
	})

	t.Run("modelWithNilFields", func(t *testing.T) {
		model := &Model{
			Id:                nil,
			ExternalGroupName: nil,
			RoleAssignments:   nil,
		}

		req, pe, err := NewRoleMappingRequest(model)

		require.NoError(t, err)
		require.Equal(t, handler.ProgressEvent{}, pe)
		require.NotNil(t, req)
		assert.Nil(t, req.Id)
		assert.Empty(t, req.ExternalGroupName)
		assert.Nil(t, req.RoleAssignments)
	})
}

func TestExpandRoleAssignments(t *testing.T) {
	t.Run("withAssignments", func(t *testing.T) {
		assignments := []RoleAssignment{
			{GroupId: util.StringPtr("group1"), OrgId: util.StringPtr("org1"), Role: util.StringPtr("ROLE1")},
			{GroupId: util.StringPtr("group2"), OrgId: util.StringPtr("org2"), Role: util.StringPtr("ROLE2")},
		}

		result := expandRoleAssignments(assignments)

		assert.Equal(t, 2, len(result))
		assert.Equal(t, "org1", *result[0].OrgId)
		assert.Equal(t, "group1", *result[0].GroupId)
		assert.Equal(t, "ROLE1", *result[0].Role)
	})

	t.Run("withNilRole", func(t *testing.T) {
		assignments := []RoleAssignment{
			{GroupId: util.StringPtr("group1"), OrgId: util.StringPtr("org1"), Role: nil},
		}

		result := expandRoleAssignments(assignments)

		assert.Equal(t, 0, len(result))
	})

	t.Run("emptyAssignments", func(t *testing.T) {
		assignments := []RoleAssignment{}

		result := expandRoleAssignments(assignments)

		assert.Equal(t, 0, len(result))
	})

	t.Run("nilFields", func(t *testing.T) {
		assignments := []RoleAssignment{
			{GroupId: nil, OrgId: nil, Role: nil},
		}

		result := expandRoleAssignments(assignments)

		assert.Equal(t, 0, len(result))
	})

	t.Run("sorting", func(t *testing.T) {
		assignments := []RoleAssignment{
			{GroupId: util.StringPtr("group2"), OrgId: util.StringPtr("org2"), Role: util.StringPtr("ROLE2")},
			{GroupId: util.StringPtr("group1"), OrgId: util.StringPtr("org1"), Role: util.StringPtr("ROLE1")},
		}

		result := expandRoleAssignments(assignments)

		assert.Equal(t, 2, len(result))
		// Should be sorted by org_id, then group_id, then role
		assert.Equal(t, "org1", *result[0].OrgId)
		assert.Equal(t, "org2", *result[1].OrgId)
	})
}

func TestFlattenRoleAssignments(t *testing.T) {
	t.Run("withAssignments", func(t *testing.T) {
		assignments := []admin20250312010.ConnectedOrgConfigRoleAssignment{
			{GroupId: util.StringPtr("group1"), OrgId: util.StringPtr("org1"), Role: util.StringPtr("ROLE1")},
			{GroupId: util.StringPtr("group2"), OrgId: util.StringPtr("org2"), Role: util.StringPtr("ROLE2")},
		}

		result := flattenRoleAssignments(assignments)

		assert.Equal(t, 2, len(result))
		assert.Equal(t, "org1", *result[0].OrgId)
		assert.Equal(t, "group1", *result[0].GroupId)
		assert.Equal(t, "ROLE1", *result[0].Role)
	})

	t.Run("emptyAssignments", func(t *testing.T) {
		assignments := []admin20250312010.ConnectedOrgConfigRoleAssignment{}

		result := flattenRoleAssignments(assignments)

		assert.Nil(t, result)
	})

	t.Run("nilAssignments", func(t *testing.T) {
		var assignments []admin20250312010.ConnectedOrgConfigRoleAssignment

		result := flattenRoleAssignments(assignments)

		assert.Nil(t, result)
	})

	t.Run("sorting", func(t *testing.T) {
		assignments := []admin20250312010.ConnectedOrgConfigRoleAssignment{
			{GroupId: util.StringPtr("group2"), OrgId: util.StringPtr("org2"), Role: util.StringPtr("ROLE2")},
			{GroupId: util.StringPtr("group1"), OrgId: util.StringPtr("org1"), Role: util.StringPtr("ROLE1")},
		}

		result := flattenRoleAssignments(assignments)

		assert.Equal(t, 2, len(result))
		// Should be sorted
		assert.Equal(t, "org1", *result[0].OrgId)
		assert.Equal(t, "org2", *result[1].OrgId)
	})
}

func TestRoleAssignmentsEqual(t *testing.T) {
	t.Run("equalAssignments", func(t *testing.T) {
		prev := []RoleAssignment{
			{GroupId: util.StringPtr("group1"), OrgId: util.StringPtr("org1"), Role: util.StringPtr("ROLE1")},
			{GroupId: util.StringPtr("group2"), OrgId: util.StringPtr("org2"), Role: util.StringPtr("ROLE2")},
		}
		current := []RoleAssignment{
			{GroupId: util.StringPtr("group2"), OrgId: util.StringPtr("org2"), Role: util.StringPtr("ROLE2")},
			{GroupId: util.StringPtr("group1"), OrgId: util.StringPtr("org1"), Role: util.StringPtr("ROLE1")},
		}

		result := roleAssignmentsEqual(prev, current)

		assert.True(t, result)
	})

	t.Run("differentLength", func(t *testing.T) {
		prev := []RoleAssignment{
			{GroupId: util.StringPtr("group1"), OrgId: util.StringPtr("org1"), Role: util.StringPtr("ROLE1")},
		}
		current := []RoleAssignment{
			{GroupId: util.StringPtr("group1"), OrgId: util.StringPtr("org1"), Role: util.StringPtr("ROLE1")},
			{GroupId: util.StringPtr("group2"), OrgId: util.StringPtr("org2"), Role: util.StringPtr("ROLE2")},
		}

		result := roleAssignmentsEqual(prev, current)

		assert.False(t, result)
	})

	t.Run("differentAssignments", func(t *testing.T) {
		prev := []RoleAssignment{
			{GroupId: util.StringPtr("group1"), OrgId: util.StringPtr("org1"), Role: util.StringPtr("ROLE1")},
		}
		current := []RoleAssignment{
			{GroupId: util.StringPtr("group1"), OrgId: util.StringPtr("org1"), Role: util.StringPtr("ROLE2")},
		}

		result := roleAssignmentsEqual(prev, current)

		assert.False(t, result)
	})

	t.Run("emptyAssignments", func(t *testing.T) {
		prev := []RoleAssignment{}
		current := []RoleAssignment{}

		result := roleAssignmentsEqual(prev, current)

		assert.True(t, result)
	})

	t.Run("nilAssignments", func(t *testing.T) {
		var prev []RoleAssignment
		var current []RoleAssignment

		result := roleAssignmentsEqual(prev, current)

		assert.True(t, result)
	})

	t.Run("missingKey", func(t *testing.T) {
		prev := []RoleAssignment{
			{GroupId: util.StringPtr("group1"), OrgId: util.StringPtr("org1"), Role: util.StringPtr("ROLE1")},
		}
		current := []RoleAssignment{
			{GroupId: util.StringPtr("group2"), OrgId: util.StringPtr("org2"), Role: util.StringPtr("ROLE2")},
		}

		result := roleAssignmentsEqual(prev, current)

		assert.False(t, result)
	})

	t.Run("differentSetLengths", func(t *testing.T) {
		// This tests the edge case where sets might have different lengths
		// (though this shouldn't happen with unique keys, it tests the code path)
		prev := []RoleAssignment{
			{GroupId: util.StringPtr("group1"), OrgId: util.StringPtr("org1"), Role: util.StringPtr("ROLE1")},
			{GroupId: util.StringPtr("group1"), OrgId: util.StringPtr("org1"), Role: util.StringPtr("ROLE1")}, // duplicate
		}
		current := []RoleAssignment{
			{GroupId: util.StringPtr("group1"), OrgId: util.StringPtr("org1"), Role: util.StringPtr("ROLE1")},
		}

		result := roleAssignmentsEqual(prev, current)

		// Should be false because prev has 2 items but current has 1
		assert.False(t, result)
	})
}

func TestHasExternalGroupNameChanged(t *testing.T) {
	t.Run("prevModelNil", func(t *testing.T) {
		currentModel := &Model{
			ExternalGroupName: util.StringPtr("test"),
		}

		result := hasExternalGroupNameChanged(nil, currentModel)

		assert.True(t, result)
	})

	t.Run("prevModelNilCurrentNil", func(t *testing.T) {
		currentModel := &Model{
			ExternalGroupName: nil,
		}

		result := hasExternalGroupNameChanged(nil, currentModel)

		assert.False(t, result)
	})

	t.Run("sameValue", func(t *testing.T) {
		name := "test"
		prevModel := &Model{
			ExternalGroupName: &name,
		}
		currentModel := &Model{
			ExternalGroupName: &name,
		}

		result := hasExternalGroupNameChanged(prevModel, currentModel)

		assert.False(t, result)
	})

	t.Run("differentValue", func(t *testing.T) {
		name1 := "test1"
		name2 := "test2"
		prevModel := &Model{
			ExternalGroupName: &name1,
		}
		currentModel := &Model{
			ExternalGroupName: &name2,
		}

		result := hasExternalGroupNameChanged(prevModel, currentModel)

		assert.True(t, result)
	})

	t.Run("prevNilCurrentNotNil", func(t *testing.T) {
		name := "test"
		prevModel := &Model{
			ExternalGroupName: nil,
		}
		currentModel := &Model{
			ExternalGroupName: &name,
		}

		result := hasExternalGroupNameChanged(prevModel, currentModel)

		assert.True(t, result)
	})
}

func TestRoleAssignmentKey(t *testing.T) {
	t.Run("allFieldsPresent", func(t *testing.T) {
		ra := RoleAssignment{
			GroupId: util.StringPtr("group1"),
			OrgId:   util.StringPtr("org1"),
			Role:    util.StringPtr("ROLE1"),
		}

		result := roleAssignmentKey(ra)

		assert.Equal(t, "group1|org1|ROLE1", result)
	})

	t.Run("nilFields", func(t *testing.T) {
		ra := RoleAssignment{
			GroupId: nil,
			OrgId:   nil,
			Role:    nil,
		}

		result := roleAssignmentKey(ra)

		assert.Equal(t, "||", result)
	})

	t.Run("partialFields", func(t *testing.T) {
		ra := RoleAssignment{
			GroupId: util.StringPtr("group1"),
			OrgId:   nil,
			Role:    util.StringPtr("ROLE1"),
		}

		result := roleAssignmentKey(ra)

		assert.Equal(t, "group1||ROLE1", result)
	})
}

func TestRoleAssignmentGroupKey(t *testing.T) {
	t.Run("allFieldsPresent", func(t *testing.T) {
		ra := RoleAssignment{
			GroupId: util.StringPtr("507f1f77bcf86cd799439013"),
			OrgId:   util.StringPtr("507f1f77bcf86cd799439012"),
			Role:    util.StringPtr("ORG_MEMBER"),
		}

		result := roleAssignmentGroupKey(ra)

		assert.Equal(t, "507f1f77bcf86cd799439013|507f1f77bcf86cd799439012", result)
	})

	t.Run("nilFields", func(t *testing.T) {
		ra := RoleAssignment{
			GroupId: nil,
			OrgId:   nil,
			Role:    nil,
		}

		result := roleAssignmentGroupKey(ra)

		assert.Equal(t, "|", result)
	})

	t.Run("partialFields", func(t *testing.T) {
		ra := RoleAssignment{
			GroupId: util.StringPtr("507f1f77bcf86cd799439013"),
			OrgId:   nil,
			Role:    util.StringPtr("ORG_MEMBER"),
		}

		result := roleAssignmentGroupKey(ra)

		assert.Equal(t, "507f1f77bcf86cd799439013|", result)
	})
}

func TestMRoleAssignmentSorting(t *testing.T) {
	t.Run("sortByOrgId", func(t *testing.T) {
		ra := mRoleAssignment{
			{OrgId: util.StringPtr("org2"), GroupId: util.StringPtr("group1"), Role: util.StringPtr("ROLE1")},
			{OrgId: util.StringPtr("org1"), GroupId: util.StringPtr("group1"), Role: util.StringPtr("ROLE1")},
		}

		sort.Sort(ra)

		assert.Equal(t, "org1", *ra[0].OrgId)
		assert.Equal(t, "org2", *ra[1].OrgId)
	})

	t.Run("sortByGroupId", func(t *testing.T) {
		ra := mRoleAssignment{
			{OrgId: util.StringPtr("org1"), GroupId: util.StringPtr("group2"), Role: util.StringPtr("ROLE1")},
			{OrgId: util.StringPtr("org1"), GroupId: util.StringPtr("group1"), Role: util.StringPtr("ROLE1")},
		}

		sort.Sort(ra)

		assert.Equal(t, "group1", *ra[0].GroupId)
		assert.Equal(t, "group2", *ra[1].GroupId)
	})

	t.Run("sortByRole", func(t *testing.T) {
		ra := mRoleAssignment{
			{OrgId: util.StringPtr("org1"), GroupId: util.StringPtr("group1"), Role: util.StringPtr("ROLE2")},
			{OrgId: util.StringPtr("org1"), GroupId: util.StringPtr("group1"), Role: util.StringPtr("ROLE1")},
		}

		sort.Sort(ra)

		assert.Equal(t, "ROLE1", *ra[0].Role)
		assert.Equal(t, "ROLE2", *ra[1].Role)
	})

	t.Run("sortComplex", func(t *testing.T) {
		ra := mRoleAssignment{
			{OrgId: util.StringPtr("org2"), GroupId: util.StringPtr("group2"), Role: util.StringPtr("ROLE2")},
			{OrgId: util.StringPtr("org1"), GroupId: util.StringPtr("group2"), Role: util.StringPtr("ROLE1")},
			{OrgId: util.StringPtr("org1"), GroupId: util.StringPtr("group1"), Role: util.StringPtr("ROLE1")},
		}

		sort.Sort(ra)

		assert.Equal(t, "org1", *ra[0].OrgId)
		assert.Equal(t, "group1", *ra[0].GroupId)
		assert.Equal(t, "org1", *ra[1].OrgId)
		assert.Equal(t, "group2", *ra[1].GroupId)
		assert.Equal(t, "org2", *ra[2].OrgId)
	})

	t.Run("len", func(t *testing.T) {
		ra := mRoleAssignment{
			{OrgId: util.StringPtr("org1"), GroupId: util.StringPtr("group1"), Role: util.StringPtr("ROLE1")},
		}

		assert.Equal(t, 1, ra.Len())
	})

	t.Run("swap", func(t *testing.T) {
		ra := mRoleAssignment{
			{OrgId: util.StringPtr("org1"), GroupId: util.StringPtr("group1"), Role: util.StringPtr("ROLE1")},
			{OrgId: util.StringPtr("org2"), GroupId: util.StringPtr("group2"), Role: util.StringPtr("ROLE2")},
		}

		ra.Swap(0, 1)

		assert.Equal(t, "org2", *ra[0].OrgId)
		assert.Equal(t, "org1", *ra[1].OrgId)
	})
}
