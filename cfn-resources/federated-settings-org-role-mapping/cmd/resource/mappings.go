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
	"sort"
	"strings"

	admin20250312010 "go.mongodb.org/atlas-sdk/v20250312010/admin"

	"github.com/aws-cloudformation/cloudformation-cli-go-plugin/cfn/handler"
	"github.com/mongodb/mongodbatlas-cloudformation-resources/util"
)

// GetRoleMappingModel converts an Atlas SDK AuthFederationRoleMapping to a CFN Model
// Preserves all fields from currentModel (primary identifier preservation)
func GetRoleMappingModel(roleMapping *admin20250312010.AuthFederationRoleMapping, currentModel *Model) *Model {
	model := new(Model)

	if currentModel != nil {
		model = currentModel
	}

	model.Id = roleMapping.Id
	model.ExternalGroupName = &roleMapping.ExternalGroupName
	model.RoleAssignments = flattenRoleAssignments(roleMapping.GetRoleAssignments())

	return model
}

// NewRoleMappingRequest converts a CFN Model to an Atlas SDK AuthFederationRoleMapping request
// Note: Id is NOT included in the request (matching Terraform behavior)
// - For Create: Id doesn't exist yet, it's returned from the API
// - For Update: Id is used in the API path, not in the request body
func NewRoleMappingRequest(currentModel *Model) (*admin20250312010.AuthFederationRoleMapping, handler.ProgressEvent, error) {
	roleMappingRequest := &admin20250312010.AuthFederationRoleMapping{}
	if currentModel.ExternalGroupName != nil {
		roleMappingRequest.ExternalGroupName = *currentModel.ExternalGroupName
	}
	if currentModel.RoleAssignments != nil {
		roleAssignments := expandRoleAssignments(currentModel.RoleAssignments)
		roleMappingRequest.RoleAssignments = &roleAssignments
	}
	return roleMappingRequest, handler.ProgressEvent{}, nil
}

// mRoleAssignment is a type alias for sorting role assignments (matching Terraform behavior)
type mRoleAssignment []admin20250312010.ConnectedOrgConfigRoleAssignment

func (ra mRoleAssignment) Len() int      { return len(ra) }
func (ra mRoleAssignment) Swap(i, j int) { ra[i], ra[j] = ra[j], ra[i] }
func (ra mRoleAssignment) Less(i, j int) bool {
	// Sort by org_id first, then group_id, then role (matching Terraform behavior)
	orgIdI := util.SafeString(ra[i].OrgId)
	orgIdJ := util.SafeString(ra[j].OrgId)
	if compareVal := strings.Compare(orgIdI, orgIdJ); compareVal != 0 {
		return compareVal < 0
	}

	groupIdI := util.SafeString(ra[i].GroupId)
	groupIdJ := util.SafeString(ra[j].GroupId)
	if compareVal := strings.Compare(groupIdI, groupIdJ); compareVal != 0 {
		return compareVal < 0
	}

	roleI := util.SafeString(ra[i].Role)
	roleJ := util.SafeString(ra[j].Role)
	return roleI < roleJ
}

// expandRoleAssignments converts CFN RoleAssignments to Atlas SDK ConnectedOrgConfigRoleAssignment slice
func expandRoleAssignments(assignments []RoleAssignment) []admin20250312010.ConnectedOrgConfigRoleAssignment {
	// Pre-allocate slice with capacity matching the number of assignments
	roleAssignments := make([]admin20250312010.ConnectedOrgConfigRoleAssignment, 0, len(assignments))

	for _, assignment := range assignments {
		// Each RoleAssignment now has a single Role (matching Terraform's data source behavior)
		if assignment.Role != nil {
			roleAssignments = append(roleAssignments, admin20250312010.ConnectedOrgConfigRoleAssignment{
				OrgId:   assignment.OrgId,
				GroupId: assignment.GroupId,
				Role:    assignment.Role,
			})
		}
	}

	// Sort role assignments for consistent ordering (matching Terraform behavior)
	sort.Sort(mRoleAssignment(roleAssignments))
	return roleAssignments
}

// flattenRoleAssignments converts Atlas SDK ConnectedOrgConfigRoleAssignment slice to CFN RoleAssignments
func flattenRoleAssignments(assignments []admin20250312010.ConnectedOrgConfigRoleAssignment) []RoleAssignment {
	if len(assignments) == 0 {
		return nil
	}

	// Sort role assignments for consistent ordering (matching Terraform behavior)
	sort.Sort(mRoleAssignment(assignments))

	// Create one RoleAssignment per API role assignment (matching Terraform's FlattenRoleAssignments data source behavior)
	// Each role assignment from the API becomes a separate entry with a single Role
	flattenedRoleAssignments := make([]RoleAssignment, 0, len(assignments))

	for _, row := range assignments {
		flattenedRoleAssignments = append(flattenedRoleAssignments, RoleAssignment{
			OrgId:   row.OrgId,
			GroupId: row.GroupId,
			Role:    row.Role,
		})
	}

	return flattenedRoleAssignments
}

// roleAssignmentsEqual compares two role assignment slices for equality
// This is used to detect changes in Update handler (matching Terraform's HasChange behavior)
func roleAssignmentsEqual(prev, current []RoleAssignment) bool {
	if len(prev) != len(current) {
		return false
	}
	if len(prev) == 0 {
		return true
	}

	// Create sets for comparison (order doesn't matter)
	// Key format: "orgId|groupId|role" for unique identification
	prevSet := make(map[string]bool)
	for _, ra := range prev {
		key := roleAssignmentKey(ra)
		prevSet[key] = true
	}

	currentSet := make(map[string]bool)
	for _, ra := range current {
		key := roleAssignmentKey(ra)
		currentSet[key] = true
	}

	// Compare the sets
	if len(prevSet) != len(currentSet) {
		return false
	}

	for key := range prevSet {
		if !currentSet[key] {
			return false
		}
	}

	return true
}

// hasExternalGroupNameChanged checks if the external group name has changed between models
func hasExternalGroupNameChanged(prevModel, currentModel *Model) bool {
	if prevModel == nil {
		return currentModel.ExternalGroupName != nil
	}
	return !util.AreStringPtrEqual(prevModel.ExternalGroupName, currentModel.ExternalGroupName)
}

// roleAssignmentKey creates a unique key for a role assignment (org_id|group_id|role)
func roleAssignmentKey(ra RoleAssignment) string {
	return util.SafeString(ra.GroupId) + "|" + util.SafeString(ra.OrgId) + "|" + util.SafeString(ra.Role)
}

// roleAssignmentGroupKey creates a unique key for grouping role assignments by org_id/group_id
// (kept for backward compatibility, but not used in current implementation)
func roleAssignmentGroupKey(ra RoleAssignment) string {
	return util.SafeString(ra.GroupId) + "|" + util.SafeString(ra.OrgId)
}
