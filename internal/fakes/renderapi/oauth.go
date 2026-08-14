package renderapi

import (
	"net/http"
	"strings"
	"time"
)

// OAuthRevokeRequest records a token revocation attempt received by the fake.
type OAuthRevokeRequest struct {
	AccessToken string
}

type oauthResponse struct {
	status int
	delay  time.Duration
}

// OAuthResource holds recorded OAuth requests and response configuration.
// Each kind of request the fake records gets its own store, so adding one
// (a device grant, a token refresh) leaves the existing stores alone.
type OAuthResource struct {
	Revokes   Resource[OAuthRevokeRequest]
	responses []oauthResponse
}

// RespondWith queues the status and optional delay for the next token
// revocation request. Revocation succeeds with 204 No Content by default.
func (r *OAuthResource) RespondWith(status int, delay ...time.Duration) {
	response := oauthResponse{status: status}
	if len(delay) > 0 {
		response.delay = delay[0]
	}
	r.responses = append(r.responses, response)
}

func (r *OAuthResource) nextResponse() oauthResponse {
	if len(r.responses) == 0 {
		return oauthResponse{status: http.StatusNoContent}
	}
	response := r.responses[0]
	r.responses = r.responses[1:]
	return response
}

func registerOAuthRoutes(mux *http.ServeMux, s *Server, record func(*http.Request)) {
	mux.HandleFunc("POST /oauth/revoke", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		s.OAuth.Revokes.Add(OAuthRevokeRequest{
			AccessToken: strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "),
		})

		response := s.OAuth.nextResponse()
		time.Sleep(response.delay)
		w.WriteHeader(response.status)
	})
}
