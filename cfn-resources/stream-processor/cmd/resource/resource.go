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
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"time"

	"github.com/aws-cloudformation/cloudformation-cli-go-plugin/cfn/handler"

	"github.com/mongodb/mongodbatlas-cloudformation-resources/util"
	"github.com/mongodb/mongodbatlas-cloudformation-resources/util/constants"
	"github.com/mongodb/mongodbatlas-cloudformation-resources/util/logger"
	"github.com/mongodb/mongodbatlas-cloudformation-resources/util/progressevent"
	"github.com/mongodb/mongodbatlas-cloudformation-resources/util/validator"
	admin20250312010 "go.mongodb.org/atlas-sdk/v20250312010/admin"
)

const (
	InitiatingState = "INIT"
	CreatingState   = "CREATING"
	CreatedState    = "CREATED"
	StartedState    = "STARTED"
	StoppedState    = "STOPPED"
	DroppedState    = "DROPPED"
	FailedState     = "FAILED"
)

const (
	defaultCallbackDelaySeconds = 3
	defaultCreateTimeout        = 20 * time.Minute // Default 20 minutes like Terraform
)

func setup() {
	util.SetupLogger("mongodb-atlas-stream-processor")
}

var CreateRequiredFields = []string{constants.ProjectID, constants.ProcessorName, constants.Pipeline}
var ReadRequiredFields = []string{constants.ProjectID, constants.ProcessorName}
var UpdateRequiredFields = []string{constants.ProjectID, constants.ProcessorName, constants.Pipeline}
var DeleteRequiredFields = []string{constants.ProjectID, constants.ProcessorName}

// initEnvWithLatestClient is a variable that can be swapped in tests for mocking
var initEnvWithLatestClient = func(req handler.Request, currentModel *Model, requiredFields []string) (*admin20250312010.APIClient, *handler.ProgressEvent) {
	setup()
	util.SetDefaultProfileIfNotDefined(&currentModel.Profile)

	if errEvent := validator.ValidateModel(requiredFields, currentModel); errEvent != nil {
		return nil, errEvent
	}

	client, peErr := util.NewAtlasClient(&req, currentModel.Profile)
	if peErr != nil {
		return nil, peErr
	}
	return client.AtlasSDK, nil
}

// callbackData holds values stored in CloudFormation callback context
type callbackData struct {
	ProjectID               string
	WorkspaceOrInstanceName string
	ProcessorName           string
	NeedsStarting           bool
	PlannedState            string
	StartTime               string // ISO 8601 timestamp when operation started
	TimeoutDuration         string // Duration string (e.g., "20m")
	DeleteOnCreateTimeout   bool
}

// isCallback checks if this is a callback request
func isCallback(req *handler.Request) bool {
	_, found := req.CallbackContext["callbackStreamProcessor"]
	return found
}

// getCallbackData extracts values from the request's callback context
func getCallbackData(req handler.Request) *callbackData {
	ctx := &callbackData{}

	// Extract values from callback context
	if val, ok := req.CallbackContext["projectID"].(string); ok {
		ctx.ProjectID = val
	}
	if val, ok := req.CallbackContext["workspaceName"].(string); ok {
		ctx.WorkspaceOrInstanceName = val
	}
	if val, ok := req.CallbackContext["processorName"].(string); ok {
		ctx.ProcessorName = val
	}
	if val, ok := req.CallbackContext["needsStarting"].(bool); ok {
		ctx.NeedsStarting = val
	}
	if val, ok := req.CallbackContext["plannedState"].(string); ok {
		ctx.PlannedState = val
	}
	if val, ok := req.CallbackContext["startTime"].(string); ok {
		ctx.StartTime = val
	}
	if val, ok := req.CallbackContext["timeoutDuration"].(string); ok {
		ctx.TimeoutDuration = val
	}
	if val, ok := req.CallbackContext["deleteOnCreateTimeout"].(bool); ok {
		ctx.DeleteOnCreateTimeout = val
	}

	return ctx
}

// validateCallbackData ensures all required callback values are present
func validateCallbackData(ctx *callbackData) *handler.ProgressEvent {
	if ctx.ProjectID == "" || ctx.WorkspaceOrInstanceName == "" || ctx.ProcessorName == "" {
		return &handler.ProgressEvent{
			OperationStatus: handler.Failed,
			Message:         "Missing required values in callback context",
		}
	}
	return nil
}

// copyIdentifyingFields copies identifying fields from currentModel to resourceModel
// This ensures CloudFormation can properly track the resource using primaryIdentifier fields
func copyIdentifyingFields(resourceModel, currentModel *Model) {
	resourceModel.Profile = currentModel.Profile
	resourceModel.ProjectId = currentModel.ProjectId
	resourceModel.ProcessorName = currentModel.ProcessorName

	if currentModel.WorkspaceName != nil && *currentModel.WorkspaceName != "" {
		resourceModel.WorkspaceName = currentModel.WorkspaceName
		resourceModel.InstanceName = nil
	} else {
		resourceModel.InstanceName = currentModel.InstanceName
		resourceModel.WorkspaceName = nil
	}
}

// buildCallbackContext creates a callback context map for InProgress events
func buildCallbackContext(projectID, workspaceOrInstanceName, processorName string, additionalFields map[string]any) map[string]any {
	ctx := map[string]any{
		"callbackStreamProcessor": true, // Callback marker (similar to cluster's pattern)
		"projectID":               projectID,
		"workspaceName":           workspaceOrInstanceName,
		"processorName":           processorName,
	}

	// Merge additional fields
	maps.Copy(ctx, additionalFields)

	return ctx
}

// parseTimeout parses a timeout string (e.g., "20m", "10s") and returns duration
func parseTimeout(timeoutStr string) time.Duration {
	if timeoutStr == "" {
		return defaultCreateTimeout
	}
	duration, err := time.ParseDuration(timeoutStr)
	if err != nil {
		_, _ = logger.Warnf("Invalid timeout format '%s', using default: %v", timeoutStr, err)
		return defaultCreateTimeout
	}
	return duration
}

// isTimeoutExceeded checks if the timeout has been exceeded
func isTimeoutExceeded(startTimeStr, timeoutDurationStr string) bool {
	if startTimeStr == "" || timeoutDurationStr == "" {
		return false
	}

	startTime, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		_, _ = logger.Warnf("Invalid start time format '%s': %v", startTimeStr, err)
		return false
	}

	timeoutDuration := parseTimeout(timeoutDurationStr)
	elapsed := time.Since(startTime)

	return elapsed >= timeoutDuration
}

// cleanupOnCreateTimeout deletes the resource if timeout occurred and DeleteOnCreateTimeout is true
func cleanupOnCreateTimeout(ctx context.Context, atlasClient *admin20250312010.APIClient, callbackCtx *callbackData) error {
	if !callbackCtx.DeleteOnCreateTimeout {
		return nil
	}

	_, err := atlasClient.StreamsApi.DeleteStreamProcessor(ctx, callbackCtx.ProjectID, callbackCtx.WorkspaceOrInstanceName, callbackCtx.ProcessorName).Execute()
	if err != nil {
		// Log but don't fail - cleanup is best effort
		_, _ = logger.Warnf("Cleanup delete failed: %v", err)
	}
	return nil
}

func Create(req handler.Request, prevModel *Model, currentModel *Model) (handler.ProgressEvent, error) {
	// Initial create: full validation and initialization
	atlasClient, peErr := initEnvWithLatestClient(req, currentModel, CreateRequiredFields)
	if peErr != nil {
		return *peErr, nil
	}

	// Check if this is a callback (similar to cluster's pattern)
	if isCallback(&req) {
		callbackCtx := getCallbackData(req)
		if peErr := validateCallbackData(callbackCtx); peErr != nil {
			return *peErr, nil
		}
		return handleCreateCallback(
			context.Background(),
			atlasClient,
			currentModel,
			callbackCtx,
		)
	}

	workspaceOrInstanceName, err := GetWorkspaceOrInstanceName(currentModel)
	if err != nil {
		return handler.ProgressEvent{
			OperationStatus: handler.Failed,
			Message:         err.Error(),
		}, nil
	}

	ctx := context.Background()
	projectID := util.SafeString(currentModel.ProjectId)
	processorName := util.SafeString(currentModel.ProcessorName)

	// Initial create - validate state if provided
	var needsStarting bool
	if currentModel.State != nil {
		state := *currentModel.State
		switch state {
		case StartedState:
			needsStarting = true
		case CreatedState:
			needsStarting = false
		default:
			return handler.ProgressEvent{
				OperationStatus: handler.Failed,
				Message:         "When creating a stream processor, the only valid states are CREATED and STARTED",
			}, nil
		}
	}

	// Initial create - create the stream processor
	streamProcessorReq, err := NewStreamProcessorReq(currentModel)
	if err != nil {
		return handler.ProgressEvent{
			OperationStatus: handler.Failed,
			Message:         fmt.Sprintf("Error creating stream processor request: %s", err.Error()),
		}, nil
	}

	_, apiResp, err := atlasClient.StreamsApi.CreateStreamProcessor(ctx, projectID, workspaceOrInstanceName, streamProcessorReq).Execute()
	if err != nil {
		return handleError(apiResp, constants.CREATE, err)
	}

	// Get timeout configuration
	timeoutStr := ""
	if currentModel.Timeouts != nil && currentModel.Timeouts.Create != nil {
		timeoutStr = *currentModel.Timeouts.Create
	}

	deleteOnCreateTimeout := true // Default to true like Terraform
	if currentModel.DeleteOnCreateTimeout != nil {
		deleteOnCreateTimeout = *currentModel.DeleteOnCreateTimeout
	}

	// Return InProgress to wait for CREATED state
	return handler.ProgressEvent{
		OperationStatus:      handler.InProgress,
		Message:              "Creating stream processor",
		ResourceModel:        currentModel,
		CallbackDelaySeconds: defaultCallbackDelaySeconds,
		CallbackContext: buildCallbackContext(projectID, workspaceOrInstanceName, processorName, map[string]any{
			"needsStarting":         needsStarting,
			"startTime":             time.Now().Format(time.RFC3339),
			"timeoutDuration":       timeoutStr,
			"deleteOnCreateTimeout": deleteOnCreateTimeout,
		}),
	}, nil
}

func handleCreateCallback(ctx context.Context, atlasClient *admin20250312010.APIClient, currentModel *Model, callbackCtx *callbackData) (handler.ProgressEvent, error) {
	needsStarting := callbackCtx.NeedsStarting

	// Check for timeout
	if isTimeoutExceeded(callbackCtx.StartTime, callbackCtx.TimeoutDuration) {
		// Timeout occurred - handle cleanup if enabled
		if err := cleanupOnCreateTimeout(context.Background(), atlasClient, callbackCtx); err != nil {
			return handler.ProgressEvent{
				OperationStatus: handler.Failed,
				Message:         fmt.Sprintf("Timeout reached and cleanup failed: %s", err.Error()),
			}, nil
		}
		cleanupMsg := "Timeout reached when waiting for stream processor creation"
		if callbackCtx.DeleteOnCreateTimeout {
			cleanupMsg += ". Resource has been deleted because delete_on_create_timeout is true. If you suspect a transient error, wait before retrying to allow resource deletion to finish."
		} else {
			cleanupMsg += ". Cleanup was not performed because delete_on_create_timeout is false."
		}
		return handler.ProgressEvent{
			OperationStatus: handler.Failed,
			Message:         cleanupMsg,
		}, nil
	}

	streamProcessor, peErr := getStreamProcessor(ctx, atlasClient, callbackCtx.ProjectID, callbackCtx.WorkspaceOrInstanceName, callbackCtx.ProcessorName)
	if peErr != nil {
		return *peErr, nil
	}

	currentState := streamProcessor.GetState()

	// Build callback context for InProgress events
	callbackContext := buildCallbackContext(callbackCtx.ProjectID, callbackCtx.WorkspaceOrInstanceName, callbackCtx.ProcessorName, map[string]any{
		"needsStarting":         callbackCtx.NeedsStarting,
		"startTime":             callbackCtx.StartTime,
		"timeoutDuration":       callbackCtx.TimeoutDuration,
		"deleteOnCreateTimeout": callbackCtx.DeleteOnCreateTimeout,
	})

	// State-based logic: current state tells us what to do next
	switch currentState {
	case CreatedState:
		if needsStarting {
			// Start the processor
			if peErr := startStreamProcessor(ctx, atlasClient, callbackCtx.ProjectID, callbackCtx.WorkspaceOrInstanceName, callbackCtx.ProcessorName); peErr != nil {
				return *peErr, nil
			}
			// Continue waiting for STARTED state
			return createInProgressEvent("Starting stream processor", currentModel, callbackContext), nil
		}
		// No need to start, we're done
		return finalizeModel(streamProcessor, currentModel, "Create Complete")

	case StartedState:
		// Already started, we're done
		return finalizeModel(streamProcessor, currentModel, "Create Complete")

	case InitiatingState, CreatingState:
		// Still creating, continue waiting
		return createInProgressEvent(fmt.Sprintf("Creating stream processor (current state: %s)", currentState), currentModel, callbackContext), nil

	case FailedState:
		return handler.ProgressEvent{
			OperationStatus: handler.Failed,
			Message:         "Stream processor entered FAILED state",
		}, nil

	default:
		return handler.ProgressEvent{
			OperationStatus: handler.Failed,
			Message:         fmt.Sprintf("Unexpected state during creation: %s", currentState),
		}, nil
	}
}

// finalizeModel converts the stream processor to a model and returns a success event
func finalizeModel(streamProcessor *admin20250312010.StreamsProcessorWithStats, currentModel *Model, message string) (handler.ProgressEvent, error) {
	resourceModel, err := GetStreamProcessorModel(streamProcessor, currentModel)
	if err != nil {
		return handler.ProgressEvent{
			OperationStatus: handler.Failed,
			Message:         fmt.Sprintf("Error converting stream processor model: %s", err.Error()),
		}, nil
	}

	copyIdentifyingFields(resourceModel, currentModel)

	return handler.ProgressEvent{
		OperationStatus: handler.Success,
		Message:         message,
		ResourceModel:   resourceModel,
	}, nil
}

func Read(req handler.Request, prevModel *Model, currentModel *Model) (handler.ProgressEvent, error) {
	atlasClient, peErr := initEnvWithLatestClient(req, currentModel, ReadRequiredFields)
	if peErr != nil {
		return *peErr, nil
	}

	workspaceOrInstanceName, err := GetWorkspaceOrInstanceName(currentModel)
	if err != nil {
		return handler.ProgressEvent{
			OperationStatus: handler.Failed,
			Message:         err.Error(),
		}, nil
	}

	projectID := util.SafeString(currentModel.ProjectId)
	processorName := util.SafeString(currentModel.ProcessorName)

	streamProcessor, apiResp, err := atlasClient.StreamsApi.GetStreamProcessorWithParams(context.Background(),
		&admin20250312010.GetStreamProcessorApiParams{
			GroupId:       projectID,
			TenantName:    workspaceOrInstanceName,
			ProcessorName: processorName,
		}).Execute()
	if err != nil {
		if apiResp != nil && apiResp.StatusCode == http.StatusNotFound {
			// Note: Terraform removes resource from state on 404, but CloudFormation doesn't support state removal
			// Return NotFound error (CloudFormation will handle this appropriately)
			return handler.ProgressEvent{
				OperationStatus:  handler.Failed,
				Message:          "Resource not found",
				HandlerErrorCode: "NotFound",
			}, nil
		}
		return handleError(apiResp, constants.READ, err)
	}

	resourceModel, err := GetStreamProcessorModel(streamProcessor, currentModel)
	if err != nil {
		return handler.ProgressEvent{
			OperationStatus: handler.Failed,
			Message:         fmt.Sprintf("Error converting stream processor model: %s", err.Error()),
		}, nil
	}

	copyIdentifyingFields(resourceModel, currentModel)

	return handler.ProgressEvent{
		OperationStatus: handler.Success,
		Message:         "Read Complete",
		ResourceModel:   resourceModel,
	}, nil
}

func Update(req handler.Request, prevModel *Model, currentModel *Model) (handler.ProgressEvent, error) {
	// Initial update: full validation and initialization
	atlasClient, peErr := initEnvWithLatestClient(req, currentModel, UpdateRequiredFields)
	if peErr != nil {
		return *peErr, nil
	}

	// Check if this is a callback (similar to cluster's pattern)
	if isCallback(&req) {
		callbackCtx := getCallbackData(req)
		if peErr := validateCallbackData(callbackCtx); peErr != nil {
			return *peErr, nil
		}
		return handleUpdateCallback(
			context.Background(),
			atlasClient,
			currentModel,
			callbackCtx,
		)
	}

	workspaceOrInstanceName, err := GetWorkspaceOrInstanceName(currentModel)
	if err != nil {
		return handler.ProgressEvent{
			OperationStatus: handler.Failed,
			Message:         err.Error(),
		}, nil
	}

	ctx := context.Background()
	projectID := util.SafeString(currentModel.ProjectId)
	processorName := util.SafeString(currentModel.ProcessorName)

	// Initial update - determine planned state (default to previous state if not specified)
	plannedState := CreatedState
	if currentModel.State != nil && *currentModel.State != "" {
		plannedState = *currentModel.State
	} else if prevModel != nil && prevModel.State != nil {
		plannedState = *prevModel.State
	}

	// Initial update - get current state
	requestParams := &admin20250312010.GetStreamProcessorApiParams{
		GroupId:       projectID,
		TenantName:    workspaceOrInstanceName,
		ProcessorName: processorName,
	}

	currentStreamProcessor, _, err := atlasClient.StreamsApi.GetStreamProcessorWithParams(ctx, requestParams).Execute()
	if err != nil {
		return handleError(nil, constants.READ, err)
	}

	currentState := currentStreamProcessor.GetState()

	// Validate state transition
	if errMsg, isValid := validateUpdateStateTransition(currentState, plannedState); !isValid {
		return handler.ProgressEvent{
			OperationStatus: handler.Failed,
			Message:         errMsg,
		}, nil
	}

	// Stop the processor if it's currently started
	if currentState == StartedState {
		_, err := atlasClient.StreamsApi.StopStreamProcessorWithParams(ctx,
			&admin20250312010.StopStreamProcessorApiParams{
				GroupId:       projectID,
				TenantName:    workspaceOrInstanceName,
				ProcessorName: processorName,
			},
		).Execute()
		if err != nil {
			return handler.ProgressEvent{
				OperationStatus: handler.Failed,
				Message:         fmt.Sprintf("Error stopping stream processor: %s", err.Error()),
			}, nil
		}

		// Return InProgress to wait for STOPPED state
		return handler.ProgressEvent{
			OperationStatus:      handler.InProgress,
			Message:              "Stopping stream processor",
			ResourceModel:        currentModel,
			CallbackDelaySeconds: defaultCallbackDelaySeconds,
			CallbackContext: buildCallbackContext(projectID, workspaceOrInstanceName, processorName, map[string]any{
				"plannedState": plannedState,
			}),
		}, nil
	}

	// Update the stream processor
	modifyAPIRequestParams, err := NewStreamProcessorUpdateReq(currentModel)
	if err != nil {
		return handler.ProgressEvent{
			OperationStatus: handler.Failed,
			Message:         fmt.Sprintf("Error creating update request: %s", err.Error()),
		}, nil
	}

	streamProcessorResp, apiResp, err := atlasClient.StreamsApi.UpdateStreamProcessorWithParams(ctx, modifyAPIRequestParams).Execute()
	if err != nil {
		return handleError(apiResp, constants.UPDATE, err)
	}

	// Start the processor if the desired state is started
	if plannedState == StartedState {
		_, err := atlasClient.StreamsApi.StartStreamProcessorWithParams(ctx,
			&admin20250312010.StartStreamProcessorApiParams{
				GroupId:       projectID,
				TenantName:    workspaceOrInstanceName,
				ProcessorName: processorName,
			},
		).Execute()
		if err != nil {
			return handler.ProgressEvent{
				OperationStatus: handler.Failed,
				Message:         fmt.Sprintf("Error starting stream processor: %s", err.Error()),
			}, nil
		}

		// Return InProgress to wait for STARTED state
		return handler.ProgressEvent{
			OperationStatus:      handler.InProgress,
			Message:              "Starting stream processor",
			ResourceModel:        currentModel,
			CallbackDelaySeconds: defaultCallbackDelaySeconds,
			CallbackContext: buildCallbackContext(projectID, workspaceOrInstanceName, processorName, map[string]any{
				"plannedState": plannedState,
			}),
		}, nil
	}

	// Update complete, no state change needed
	return finalizeModel(streamProcessorResp, currentModel, "Update Complete")
}

func List(req handler.Request, prevModel *Model, currentModel *Model) (handler.ProgressEvent, error) {
	return handler.ProgressEvent{}, errors.New("not implemented: list")
}

func handleUpdateCallback(ctx context.Context, atlasClient *admin20250312010.APIClient, currentModel *Model, callbackCtx *callbackData) (handler.ProgressEvent, error) {
	plannedState := callbackCtx.PlannedState
	if plannedState == "" {
		plannedState = CreatedState // Default
	}

	streamProcessor, peErr := getStreamProcessor(ctx, atlasClient, callbackCtx.ProjectID, callbackCtx.WorkspaceOrInstanceName, callbackCtx.ProcessorName)
	if peErr != nil {
		return *peErr, nil
	}

	currentState := streamProcessor.GetState()

	// Build callback context for InProgress events
	callbackContext := buildCallbackContext(callbackCtx.ProjectID, callbackCtx.WorkspaceOrInstanceName, callbackCtx.ProcessorName, map[string]any{
		"plannedState": plannedState,
	})

	// State-based logic: current state tells us what to do next
	switch currentState {
	case StoppedState, CreatedState:
		// Processor is stopped/created, check if we need to update or start
		// If we're here from a callback, we likely just stopped it, so update it
		modifyAPIRequestParams, err := NewStreamProcessorUpdateReq(currentModel)
		if err != nil {
			return handler.ProgressEvent{
				OperationStatus: handler.Failed,
				Message:         fmt.Sprintf("Error creating update request: %s", err.Error()),
			}, nil
		}

		streamProcessorResp, apiResp, err := atlasClient.StreamsApi.UpdateStreamProcessorWithParams(ctx, modifyAPIRequestParams).Execute()
		if err != nil {
			return handleError(apiResp, constants.UPDATE, err)
		}

		// Start if needed
		if plannedState == StartedState {
			if peErr := startStreamProcessor(ctx, atlasClient, callbackCtx.ProjectID, callbackCtx.WorkspaceOrInstanceName, callbackCtx.ProcessorName); peErr != nil {
				return *peErr, nil
			}
			// Continue waiting for STARTED state
			return createInProgressEvent("Starting stream processor", currentModel, callbackContext), nil
		}

		// Update complete, no state change needed
		return finalizeModel(streamProcessorResp, currentModel, "Update Complete")

	case StartedState:
		// Already in desired state (if plannedState is STARTED) or need to stop first
		if plannedState == StartedState {
			// Already started, update complete
			return finalizeModel(streamProcessor, currentModel, "Update Complete")
		}
		// Need to stop first - actually stop it to avoid infinite loop
		_, err := atlasClient.StreamsApi.StopStreamProcessorWithParams(ctx,
			&admin20250312010.StopStreamProcessorApiParams{
				GroupId:       callbackCtx.ProjectID,
				TenantName:    callbackCtx.WorkspaceOrInstanceName,
				ProcessorName: callbackCtx.ProcessorName,
			},
		).Execute()
		if err != nil {
			return handler.ProgressEvent{
				OperationStatus: handler.Failed,
				Message:         fmt.Sprintf("Error stopping stream processor: %s", err.Error()),
			}, nil
		}
		// Continue waiting for STOPPED state
		return createInProgressEvent("Stopping stream processor", currentModel, callbackContext), nil

	case FailedState:
		return handler.ProgressEvent{
			OperationStatus: handler.Failed,
			Message:         "Stream processor entered FAILED state",
		}, nil

	default:
		// Still transitioning (e.g., from STARTED to STOPPED)
		return createInProgressEvent(fmt.Sprintf("Updating stream processor (current state: %s)", currentState), currentModel, callbackContext), nil
	}
}

func Delete(req handler.Request, prevModel *Model, currentModel *Model) (handler.ProgressEvent, error) {
	// Initial delete: full validation and initialization
	atlasClient, peErr := initEnvWithLatestClient(req, currentModel, DeleteRequiredFields)
	if peErr != nil {
		return *peErr, nil
	}

	workspaceOrInstanceName, err := GetWorkspaceOrInstanceName(currentModel)
	if err != nil {
		return handler.ProgressEvent{
			OperationStatus: handler.Failed,
			Message:         err.Error(),
		}, nil
	}

	ctx := context.Background()
	projectID := util.SafeString(currentModel.ProjectId)
	processorName := util.SafeString(currentModel.ProcessorName)

	// Delete the processor (no verification callbacks - matches Terraform behavior)
	_, err = atlasClient.StreamsApi.DeleteStreamProcessor(ctx, projectID, workspaceOrInstanceName, processorName).Execute()
	if err != nil {
		// Treat 404 as error (matches Terraform behavior)
		return handler.ProgressEvent{
			OperationStatus: handler.Failed,
			Message:         fmt.Sprintf("Error deleting stream processor: %s", err.Error()),
		}, nil
	}

	// Return success immediately (no verification - matches Terraform)
	return handler.ProgressEvent{
		OperationStatus: handler.Success,
		Message:         "Delete Complete",
	}, nil
}

// Helper functions for callback-based state management

// getStreamProcessor retrieves a stream processor and handles errors
// Returns (processor, progressEvent) where progressEvent is nil on success
func getStreamProcessor(ctx context.Context, atlasClient *admin20250312010.APIClient, projectID, workspaceOrInstanceName, processorName string) (*admin20250312010.StreamsProcessorWithStats, *handler.ProgressEvent) {
	requestParams := &admin20250312010.GetStreamProcessorApiParams{
		GroupId:       projectID,
		TenantName:    workspaceOrInstanceName,
		ProcessorName: processorName,
	}

	streamProcessor, resp, err := atlasClient.StreamsApi.GetStreamProcessorWithParams(ctx, requestParams).Execute()
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, &handler.ProgressEvent{
				OperationStatus: handler.Failed,
				Message:         "Stream processor not found",
			}
		}
		return nil, &handler.ProgressEvent{
			OperationStatus: handler.Failed,
			Message:         fmt.Sprintf("Error getting stream processor: %s", err.Error()),
		}
	}
	return streamProcessor, nil
}

// startStreamProcessor starts a stream processor and handles errors
// Returns progressEvent which is nil on success
func startStreamProcessor(ctx context.Context, atlasClient *admin20250312010.APIClient, projectID, workspaceOrInstanceName, processorName string) *handler.ProgressEvent {
	_, err := atlasClient.StreamsApi.StartStreamProcessorWithParams(ctx,
		&admin20250312010.StartStreamProcessorApiParams{
			GroupId:       projectID,
			TenantName:    workspaceOrInstanceName,
			ProcessorName: processorName,
		},
	).Execute()
	if err != nil {
		return &handler.ProgressEvent{
			OperationStatus: handler.Failed,
			Message:         fmt.Sprintf("Error starting stream processor: %s", err.Error()),
		}
	}
	return nil
}

// createInProgressEvent creates a standardized InProgress event
func createInProgressEvent(message string, currentModel *Model, callbackContext map[string]any) handler.ProgressEvent {
	return handler.ProgressEvent{
		OperationStatus:      handler.InProgress,
		Message:              message,
		ResourceModel:        currentModel,
		CallbackDelaySeconds: defaultCallbackDelaySeconds,
		CallbackContext:      callbackContext,
	}
}

// validateUpdateStateTransition validates if a state transition is allowed
func validateUpdateStateTransition(currentState, plannedState string) (errMsg string, isValidTransition bool) {
	if currentState == plannedState {
		return "", true
	}

	if plannedState == StoppedState && currentState != StartedState {
		return fmt.Sprintf("Stream Processor must be in %s state to transition to %s state", StartedState, StoppedState), false
	}

	if plannedState == CreatedState && currentState != CreatedState {
		return fmt.Sprintf("Stream Processor cannot transition from %s to CREATED", currentState), false
	}

	return "", true
}

func handleError(response *http.Response, method constants.CfnFunctions, err error) (handler.ProgressEvent, error) {
	errMsg := fmt.Sprintf("%s error: %s", method, err.Error())
	_, _ = logger.Warn(errMsg)

	if response != nil && response.StatusCode == http.StatusConflict {
		return handler.ProgressEvent{
			OperationStatus:  handler.Failed,
			Message:          errMsg,
			HandlerErrorCode: "AlreadyExists",
		}, nil
	}

	return progressevent.GetFailedEventByResponse(errMsg, response), nil
}
