package renderapi

import (
	"net/http"

	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
)

// BlueprintResource holds the validation response returned by the fake server.
type BlueprintResource struct {
	validationResponses []blueprintValidationResponse
}

type blueprintValidationResponse struct {
	result      *bptypes.ValidateBlueprintResponse
	contentType string
	body        []byte
}

// RespondWithValidation queues a typed response from POST /blueprints/validate.
func (r *BlueprintResource) RespondWithValidation(result bptypes.ValidateBlueprintResponse) {
	r.validationResponses = append(r.validationResponses, blueprintValidationResponse{result: &result})
}

// RespondWithRawValidation queues an unparsed 200 response from POST /blueprints/validate.
func (r *BlueprintResource) RespondWithRawValidation(contentType string, body []byte) {
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

func registerBlueprintRoutes(mux *http.ServeMux, s *Server, record func(*http.Request)) {
	mux.HandleFunc("POST /blueprints/validate", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		response := s.Blueprints.nextValidationResponse()
		if response.result == nil {
			w.Header().Set("Content-Type", response.contentType)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(response.body)
			return
		}
		writeJSON(w, http.StatusOK, response.result)
	})
}
