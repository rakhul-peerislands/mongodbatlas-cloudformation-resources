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
	"encoding/json"
	"fmt"

	admin20250312010 "go.mongodb.org/atlas-sdk/v20250312010/admin"

	"github.com/mongodb/mongodbatlas-cloudformation-resources/util"
)

// GetWorkspaceOrInstanceName returns the workspace name from workspace_name or instance_name field.
// Uses precedence logic: WorkspaceName takes precedence over InstanceName if both are provided.
// This follows CFN guidelines where mutual exclusivity is handled via precedence, not validation errors.
func GetWorkspaceOrInstanceName(model *Model) (string, error) {
	// Precedence: WorkspaceName takes precedence over InstanceName
	if model.WorkspaceName != nil && *model.WorkspaceName != "" {
		return *model.WorkspaceName, nil
	}
	if model.InstanceName != nil && *model.InstanceName != "" {
		return *model.InstanceName, nil
	}
	return "", fmt.Errorf("either WorkspaceName or InstanceName must be provided")
}

// ConvertPipelineToSdk converts JSON string pipeline to SDK format ([]any)
func ConvertPipelineToSdk(pipeline string) ([]any, error) {
	var pipelineSliceOfMaps []any
	err := json.Unmarshal([]byte(pipeline), &pipelineSliceOfMaps)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal pipeline: %w", err)
	}
	return pipelineSliceOfMaps, nil
}

// ConvertPipelineToString converts SDK pipeline format ([]any) to JSON string
func ConvertPipelineToString(pipeline []any) (string, error) {
	pipelineJSON, err := json.Marshal(pipeline)
	if err != nil {
		return "", fmt.Errorf("failed to marshal pipeline: %w", err)
	}
	return string(pipelineJSON), nil
}

// ConvertStatsToString converts stats to JSON string
func ConvertStatsToString(stats any) (string, error) {
	if stats == nil {
		return "", nil
	}
	statsJSON, err := json.Marshal(stats)
	if err != nil {
		return "", fmt.Errorf("failed to marshal stats: %w", err)
	}
	return string(statsJSON), nil
}

// NewStreamProcessorReq creates an API request from CloudFormation model
func NewStreamProcessorReq(model *Model) (*admin20250312010.StreamsProcessor, error) {
	pipeline, err := ConvertPipelineToSdk(util.SafeString(model.Pipeline))
	if err != nil {
		return nil, err
	}

	streamProcessor := &admin20250312010.StreamsProcessor{
		Name:     model.ProcessorName,
		Pipeline: &pipeline,
	}

	if model.Options != nil && model.Options.Dlq != nil {
		streamProcessor.Options = &admin20250312010.StreamsOptions{
			Dlq: &admin20250312010.StreamsDLQ{
				Coll:           model.Options.Dlq.Coll,
				ConnectionName: model.Options.Dlq.ConnectionName,
				Db:             model.Options.Dlq.Db,
			},
		}
	}

	return streamProcessor, nil
}

// NewStreamProcessorUpdateReq creates an update API request from CloudFormation model
func NewStreamProcessorUpdateReq(model *Model) (*admin20250312010.UpdateStreamProcessorApiParams, error) {
	pipeline, err := ConvertPipelineToSdk(util.SafeString(model.Pipeline))
	if err != nil {
		return nil, err
	}

	workspaceOrInstanceName, err := GetWorkspaceOrInstanceName(model)
	if err != nil {
		return nil, err
	}

	streamProcessorAPIParams := &admin20250312010.UpdateStreamProcessorApiParams{
		GroupId:       util.SafeString(model.ProjectId),
		TenantName:    workspaceOrInstanceName,
		ProcessorName: util.SafeString(model.ProcessorName),
		StreamsModifyStreamProcessor: &admin20250312010.StreamsModifyStreamProcessor{
			Name:     model.ProcessorName,
			Pipeline: &pipeline,
		},
	}

	if model.Options != nil && model.Options.Dlq != nil {
		streamProcessorAPIParams.StreamsModifyStreamProcessor.Options = &admin20250312010.StreamsModifyStreamProcessorOptions{
			Dlq: &admin20250312010.StreamsDLQ{
				Coll:           model.Options.Dlq.Coll,
				ConnectionName: model.Options.Dlq.ConnectionName,
				Db:             model.Options.Dlq.Db,
			},
		}
	}

	return streamProcessorAPIParams, nil
}

// GetStreamProcessorModel converts API response to CloudFormation model
func GetStreamProcessorModel(streamProcessor *admin20250312010.StreamsProcessorWithStats, currentModel *Model) (*Model, error) {
	model := new(Model)

	if currentModel != nil {
		model = currentModel
	}

	// Set basic fields
	model.ProcessorName = util.Pointer(streamProcessor.Name)
	model.Id = util.Pointer(streamProcessor.Id)
	model.State = util.Pointer(streamProcessor.State)

	// Convert pipeline
	if streamProcessor.Pipeline != nil {
		pipelineStr, err := ConvertPipelineToString(streamProcessor.GetPipeline())
		if err != nil {
			return nil, err
		}
		model.Pipeline = &pipelineStr
	}

	// Convert stats
	if streamProcessor.Stats != nil {
		statsStr, err := ConvertStatsToString(streamProcessor.GetStats())
		if err != nil {
			return nil, err
		}
		model.Stats = &statsStr
	}

	// Convert options
	if streamProcessor.Options != nil && streamProcessor.Options.Dlq != nil {
		model.Options = &StreamsOptions{
			Dlq: &StreamsDLQ{
				Coll:           streamProcessor.Options.Dlq.Coll,
				ConnectionName: streamProcessor.Options.Dlq.ConnectionName,
				Db:             streamProcessor.Options.Dlq.Db,
			},
		}
	} else {
		// If no options in response, preserve current model's options if it exists
		if currentModel != nil && currentModel.Options != nil {
			model.Options = currentModel.Options
		}
	}

	return model, nil
}
