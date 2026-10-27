package renderapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/render-oss/cli/internal/testids"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
	"github.com/render-oss/cli/pkg/pointers"
)

// BlueprintResource holds created Blueprints and queued create/validation responses.
type BlueprintResource struct {
	Resource[*bptypes.Blueprint]
	createResponses     []blueprintCreateResponse
	validationResponses []blueprintValidationResponse
}

type blueprintCreateResponse struct {
	result  *bptypes.CreateBlueprintResponse
	status  int
	message string
}

type blueprintValidationResponse struct {
	result      *bptypes.ValidateBlueprintResponse
	contentType string
	body        []byte
}

// NewBlueprint returns a Blueprint with defaults for empty string fields.
// AutoSync is preserved, including an explicit false.
func NewBlueprint(b bptypes.Blueprint) *bptypes.Blueprint {
	if b.Id == "" {
		b.Id = testids.RandomBlueprintID()
	}
	if b.Name == "" {
		b.Name = "My Blueprint"
	}
	if b.Repo == "" {
		b.Repo = "https://github.com/example/app"
	}
	if b.Branch == "" {
		b.Branch = "main"
	}
	if b.Path == "" {
		b.Path = "render.yaml"
	}
	if b.Status == "" {
		b.Status = bptypes.StatusCreated
	}
	return &b
}

// RespondWithBodyForCreate queues the response body for the next Create Blueprint request.
// The server returns it as JSON with HTTP 201 Created and saves a new blueprint in Instances,
// including when planning failed. Planning is fixture-driven, not simulated.
// Queued responses override request validation after JSON decoding.
func (r *BlueprintResource) RespondWithBodyForCreate(result bptypes.CreateBlueprintResponse) {
	r.createResponses = append(r.createResponses, blueprintCreateResponse{result: &result})
}

// RespondWithErrorForCreate queues an HTTP error without creating a Blueprint.
// Queued responses override request validation after JSON decoding.
func (r *BlueprintResource) RespondWithErrorForCreate(status int, message string) {
	r.createResponses = append(r.createResponses, blueprintCreateResponse{status: status, message: message})
}

// RespondWithBodyForValidate queues the response body for the next Validate Blueprint request.
// The server returns it as JSON with HTTP 200 OK.
func (r *BlueprintResource) RespondWithBodyForValidate(result bptypes.ValidateBlueprintResponse) {
	r.validationResponses = append(r.validationResponses, blueprintValidationResponse{result: &result})
}

// RespondWithRawBytesForValidate queues raw response bytes for the next Validate Blueprint request.
// The server returns them with the supplied content type and HTTP 200 OK.
func (r *BlueprintResource) RespondWithRawBytesForValidate(contentType string, body []byte) {
	r.validationResponses = append(r.validationResponses, blueprintValidationResponse{
		contentType: contentType,
		body:        append([]byte(nil), body...),
	})
}

func (r *BlueprintResource) nextValidationResponse() blueprintValidationResponse {
	if len(r.validationResponses) == 0 {
		result := bptypes.ValidateBlueprintResponse{Valid: true}
		return blueprintValidationResponse{result: &result}
	}
	response := r.validationResponses[0]
	r.validationResponses = r.validationResponses[1:]
	return response
}

func (r *BlueprintResource) nextCreateResponse() (blueprintCreateResponse, bool) {
	if len(r.createResponses) == 0 {
		return blueprintCreateResponse{}, false
	}
	response := r.createResponses[0]
	r.createResponses = r.createResponses[1:]
	return response, true
}

func registerBlueprintRoutes(mux *http.ServeMux, s *Server, record func(*http.Request)) {
	mux.HandleFunc("POST /blueprints", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		s.handleCreateBlueprint(w, r)
	})

	mux.HandleFunc("POST /blueprints/validate", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		s.handleValidateBlueprint(w, r)
	})
}

func (s *Server) handleCreateBlueprint(w http.ResponseWriter, r *http.Request) {
	var body bptypes.CreateBlueprintRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid body", "")
		return
	}
	if response, queued := s.Blueprints.nextCreateResponse(); queued {
		if response.result != nil {
			s.Blueprints.Add(&response.result.Blueprint)
			writeJSON(w, http.StatusCreated, response.result)
		} else {
			writeAPIError(w, response.status, response.message, "")
		}
		return
	}
	if _, ok := s.ownerByID(body.OwnerId); !ok {
		writeAPIError(w, http.StatusNotFound, "workspace not found", "")
		return
	}
	if body.Name == "" {
		writeAPIError(w, http.StatusBadRequest, "name is required", "")
		return
	}
	if body.Source.Repo == "" {
		writeAPIError(w, http.StatusBadRequest, "source.repo is required", "")
		return
	}
	if body.ExistingResources != nil && !body.ExistingResources.Valid() {
		writeAPIError(w, http.StatusBadRequest, "existingResources must be adopt or create_new", "")
		return
	}
	blueprint := s.Blueprints.Add(NewBlueprint(bptypes.Blueprint{
		Name: body.Name, Repo: body.Source.Repo,
		Branch:   pointers.ValueOrDefault(body.Source.Branch, "main"),
		Path:     pointers.ValueOrDefault(body.Source.Path, "render.yaml"),
		AutoSync: pointers.ValueOrDefault(body.AutoSync, true),
	}))
	syncID := testids.RandomSyncID()
	writeJSON(w, http.StatusCreated, bptypes.CreateBlueprintResponse{
		Blueprint:       *blueprint,
		PlanningOutcome: bptypes.PlanningOutcomePlanned,
		Notices:         []bptypes.BlueprintNotice{},
		Sync: &bptypes.SyncWithPlan{
			Id: syncID, State: bptypes.SyncStateCreated,
			Commit:                bptypes.CommitRef{Id: "fake-commit"},
			DashboardUrl:          fmt.Sprintf("https://dashboard.render.com/blueprint/%s/sync/%s", blueprint.Id, syncID),
			RawPlan:               new(map[string]interface{}{"actions": []interface{}{}}),
			RequiredManualEnvVars: []bptypes.RequiredManualEnvVar{},
		},
	})
}

func (s *Server) handleValidateBlueprint(w http.ResponseWriter, _ *http.Request) {
	response := s.Blueprints.nextValidationResponse()
	if response.result == nil {
		w.Header().Set("Content-Type", response.contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(response.body)
		return
	}
	writeJSON(w, http.StatusOK, response.result)
}
