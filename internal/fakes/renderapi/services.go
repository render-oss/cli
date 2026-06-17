package renderapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/pointers"
)

// ServiceResource holds service state for the fake server.
// Tests can assert against Instances. Fake service constructors take attrs
// instead of [client.Service] so callers can describe each service type and
// runtime explicitly, such as web services, cron jobs, native runtimes, Docker,
// and image-backed services.
type ServiceResource struct {
	Resource[*client.Service]
	errorQueue []int
}

// RespondWith queues an HTTP status code to return on the next service
// operation handled by the fake server. The queue is drained in FIFO order.
func (s *ServiceResource) RespondWith(status int) {
	s.errorQueue = append(s.errorQueue, status)
}

func (s *ServiceResource) nextError() (int, bool) {
	if len(s.errorQueue) == 0 {
		return 0, false
	}
	status := s.errorQueue[0]
	s.errorQueue = s.errorQueue[1:]
	return status, true
}

// CommonServiceAttrs contains top-level [client.Service] fields shared by all
// fake service constructors. Service-specific fields belong in each attrs
// type's Details field.
type CommonServiceAttrs struct {
	ID            string
	Name          string
	OwnerID       string
	EnvironmentID string
	DashboardURL  string
	Repo          string
	Branch        string
	RootDir       string
	Slug          string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	AutoDeploy    client.AutoDeploy
	NotifyOnFail  client.NotifySetting
	Suspended     client.ServiceSuspended
	Suspenders    []client.SuspenderType
}

// WebServiceAttrs contains fields for constructing fake web service resources.
type WebServiceAttrs struct {
	Service CommonServiceAttrs
	Details WebServiceDetailsAttrs
}

// WebServiceDetailsAttrs contains fields for constructing fake web service details.
type WebServiceDetailsAttrs struct {
	RuntimeDetails  runtimeDetails
	Region          client.Region
	Plan            client.Plan
	HealthCheckPath string
	NumInstances    int
	URL             string
}

// BackgroundWorkerAttrs contains fields for constructing fake background worker resources.
type BackgroundWorkerAttrs struct {
	Service CommonServiceAttrs
	Details BackgroundWorkerDetailsAttrs
}

// BackgroundWorkerDetailsAttrs contains fields for constructing fake background worker details.
type BackgroundWorkerDetailsAttrs struct {
	RuntimeDetails runtimeDetails
	Region         client.Region
	Plan           client.Plan
	NumInstances   int
}

// PrivateServiceAttrs contains fields for constructing fake private service resources.
type PrivateServiceAttrs struct {
	Service CommonServiceAttrs
	Details PrivateServiceDetailsAttrs
}

// PrivateServiceDetailsAttrs contains fields for constructing fake private service details.
type PrivateServiceDetailsAttrs struct {
	RuntimeDetails runtimeDetails
	Region         client.Region
	Plan           client.Plan
	NumInstances   int
	URL            string
}

// StaticSiteAttrs contains fields for constructing fake static site resources.
type StaticSiteAttrs struct {
	Service CommonServiceAttrs
	Details StaticSiteDetailsAttrs
}

// StaticSiteDetailsAttrs contains fields for constructing fake static site details.
type StaticSiteDetailsAttrs struct {
	BuildCommand string
	PublishPath  string
	URL          string
}

// CronJobAttrs contains fields for constructing fake cron job resources.
type CronJobAttrs struct {
	Service CommonServiceAttrs
	Details CronJobDetailsAttrs
}

// CronJobDetailsAttrs contains fields for constructing fake cron job details.
type CronJobDetailsAttrs struct {
	RuntimeDetails runtimeDetails
	Region         client.Region
	Plan           client.Plan
	Schedule       string
}

type runtimeDetailsKind string

const (
	runtimeDetailsKindNative runtimeDetailsKind = "native"
	runtimeDetailsKindDocker runtimeDetailsKind = "docker"
	runtimeDetailsKindImage  runtimeDetailsKind = "image"
)

// runtimeDetails is the private runtime configuration used by services with a
// runtime, such as web services and cron jobs. Construct values with
// [NewNativeRuntimeDetails], [NewDockerRuntimeDetails], or
// [NewImageRuntimeDetails].
type runtimeDetails struct {
	kind               runtimeDetailsKind
	runtime            client.ServiceRuntime
	envSpecificDetails client.EnvSpecificDetails
	imagePath          string
}

// NativeRuntimeAttrs contains fields for constructing fake native runtime details.
type NativeRuntimeAttrs struct {
	Runtime      client.ServiceRuntime
	BuildCommand string
	StartCommand string
}

// DockerRuntimeAttrs contains fields for constructing fake Docker runtime details.
type DockerRuntimeAttrs struct {
	DockerCommand      string
	DockerContext      string
	DockerfilePath     string
	RegistryCredential *client.RegistryCredential
}

// ImageRuntimeAttrs contains fields for constructing fake image-backed runtime details.
type ImageRuntimeAttrs struct {
	ImagePath string
}

// NewWebService returns a fake web service resource.
func NewWebService(attrs WebServiceAttrs) *client.Service {
	if attrs.Service.Name == "" {
		attrs.Service.Name = "my-web-service"
	}
	runtime := runtimeDetailsOrDefault(attrs.Details.RuntimeDetails)
	var details client.Service_ServiceDetails
	must(details.FromWebServiceDetails(webServiceDetails(attrs.Details, runtime)))
	return newServiceWithRuntimeDetails(
		attrs.Service,
		client.WebService,
		details,
		runtime,
	)
}

// NewBackgroundWorker returns a fake background worker resource.
func NewBackgroundWorker(attrs BackgroundWorkerAttrs) *client.Service {
	if attrs.Service.Name == "" {
		attrs.Service.Name = "my-background-worker"
	}
	runtime := runtimeDetailsOrDefault(attrs.Details.RuntimeDetails)
	var details client.Service_ServiceDetails
	must(details.FromBackgroundWorkerDetails(backgroundWorkerDetails(attrs.Details, runtime)))
	return newServiceWithRuntimeDetails(
		attrs.Service,
		client.BackgroundWorker,
		details,
		runtime,
	)
}

// NewPrivateService returns a fake private service resource.
func NewPrivateService(attrs PrivateServiceAttrs) *client.Service {
	if attrs.Service.Name == "" {
		attrs.Service.Name = "my-private-service"
	}
	runtime := runtimeDetailsOrDefault(attrs.Details.RuntimeDetails)
	var details client.Service_ServiceDetails
	must(details.FromPrivateServiceDetails(privateServiceDetails(attrs.Details, runtime)))
	return newServiceWithRuntimeDetails(
		attrs.Service,
		client.PrivateService,
		details,
		runtime,
	)
}

// NewStaticSite returns a fake static site resource.
func NewStaticSite(attrs StaticSiteAttrs) *client.Service {
	if attrs.Service.Name == "" {
		attrs.Service.Name = "my-static-site"
	}
	var details client.Service_ServiceDetails
	must(details.FromStaticSiteDetails(staticSiteDetails(attrs.Details)))
	return newService(
		attrs.Service,
		client.StaticSite,
		details,
	)
}

// NewCronJob returns a fake cron job resource.
func NewCronJob(attrs CronJobAttrs) *client.Service {
	if attrs.Service.Name == "" {
		attrs.Service.Name = "my-cron-job"
	}
	runtime := runtimeDetailsOrDefault(attrs.Details.RuntimeDetails)
	var details client.Service_ServiceDetails
	must(details.FromCronJobDetails(cronJobDetails(attrs.Details, runtime)))
	return newServiceWithRuntimeDetails(
		attrs.Service,
		client.CronJob,
		details,
		runtime,
	)
}

// newService builds the common top-level [client.Service] wrapper for fake
// service constructors. Call exported constructors such as [NewWebService] from
// tests instead of calling this helper directly.
func newService(attrs CommonServiceAttrs, serviceType client.ServiceType, details client.Service_ServiceDetails) *client.Service {
	svc := client.Service{
		Branch:         pointers.PointerValueIfNotEmptyString(attrs.Branch),
		EnvironmentId:  pointers.PointerValueIfNotEmptyString(attrs.EnvironmentID),
		Repo:           pointers.PointerValueIfNotEmptyString(attrs.Repo),
		RootDir:        attrs.RootDir,
		ServiceDetails: details,
		Type:           serviceType,
	}

	svc.Id = attrs.ID
	if svc.Id == "" {
		svc.Id = testids.RandomServiceID()
		if serviceType == client.CronJob {
			svc.Id = testids.RandomCronJobID()
		}
	}

	svc.Name = attrs.Name

	svc.OwnerId = attrs.OwnerID

	svc.AutoDeploy = attrs.AutoDeploy
	if svc.AutoDeploy == "" {
		svc.AutoDeploy = client.AutoDeployYes
	}

	svc.NotifyOnFail = attrs.NotifyOnFail
	if svc.NotifyOnFail == "" {
		svc.NotifyOnFail = client.Default
	}

	svc.DashboardUrl = attrs.DashboardURL
	if svc.DashboardUrl == "" {
		svc.DashboardUrl = serviceDashboardURL(serviceType, svc.Id)
	}

	svc.Slug = attrs.Slug
	if svc.Slug == "" {
		svc.Slug = svc.Name
	}

	svc.Suspended = attrs.Suspended
	if svc.Suspended == "" {
		svc.Suspended = client.ServiceSuspendedNotSuspended
	}

	svc.Suspenders = attrs.Suspenders
	if svc.Suspenders == nil {
		svc.Suspenders = []client.SuspenderType{}
	}

	svc.CreatedAt = attrs.CreatedAt
	svc.UpdatedAt = attrs.UpdatedAt
	if svc.CreatedAt.IsZero() || svc.UpdatedAt.IsZero() {
		now := time.Now()
		if svc.CreatedAt.IsZero() {
			svc.CreatedAt = now
		}
		if svc.UpdatedAt.IsZero() {
			svc.UpdatedAt = now
		}
	}

	return &svc
}

// newServiceWithRuntimeDetails builds a fake service and applies any top-level
// fields implied by runtime configuration, such as ImagePath for image-backed
// services.
func newServiceWithRuntimeDetails(
	attrs CommonServiceAttrs,
	serviceType client.ServiceType,
	details client.Service_ServiceDetails,
	runtime runtimeDetails,
) *client.Service {
	svc := newService(attrs, serviceType, details)
	if runtime.kind == runtimeDetailsKindImage {
		svc.ImagePath = pointers.PointerValueIfNotEmptyString(runtime.imagePath)
	}
	return svc
}

func serviceDashboardURL(serviceType client.ServiceType, id string) string {
	switch serviceType {
	case client.BackgroundWorker:
		return "https://dashboard.render.com/worker/" + id
	case client.CronJob:
		return "https://dashboard.render.com/cron/" + id
	case client.PrivateService:
		return "https://dashboard.render.com/pserv/" + id
	case client.StaticSite:
		return "https://dashboard.render.com/static/" + id
	case client.WebService:
		return "https://dashboard.render.com/web/" + id
	default:
		return "https://dashboard.render.com/services/" + id
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// NewNativeRuntimeDetails returns fake runtime details for a native runtime service.
func NewNativeRuntimeDetails(attrs NativeRuntimeAttrs) runtimeDetails {
	runtime := nativeRuntimeOrDefault(attrs.Runtime)
	return runtimeDetails{
		kind:               runtimeDetailsKindNative,
		runtime:            runtime,
		envSpecificDetails: nativeEnvDetails(runtime, attrs.BuildCommand, attrs.StartCommand),
	}
}

// NewDockerRuntimeDetails returns fake runtime details for a Docker runtime service.
func NewDockerRuntimeDetails(attrs DockerRuntimeAttrs) runtimeDetails {
	return runtimeDetails{
		kind:               runtimeDetailsKindDocker,
		runtime:            client.ServiceRuntimeDocker,
		envSpecificDetails: dockerEnvDetails(attrs),
	}
}

// NewImageRuntimeDetails returns fake runtime details for an image-backed service.
func NewImageRuntimeDetails(attrs ImageRuntimeAttrs) runtimeDetails {
	imagePath := attrs.ImagePath
	if imagePath == "" {
		imagePath = "docker.io/render/example:latest"
	}
	return runtimeDetails{
		kind:      runtimeDetailsKindImage,
		runtime:   client.ServiceRuntimeImage,
		imagePath: imagePath,
	}
}

func runtimeDetailsOrDefault(runtime runtimeDetails) runtimeDetails {
	if runtime.kind == "" {
		return NewNativeRuntimeDetails(NativeRuntimeAttrs{})
	}
	return runtime
}

func nativeRuntimeOrDefault(runtime client.ServiceRuntime) client.ServiceRuntime {
	if runtime == "" {
		return client.ServiceRuntimeNode
	}
	return runtime
}

func regionOrDefault(region client.Region) client.Region {
	if region == "" {
		return client.Oregon
	}
	return region
}

func planOrDefault(plan client.Plan) client.Plan {
	if plan == "" {
		return client.PlanStarter
	}
	return plan
}

func numInstancesOrDefault(numInstances int) int {
	if numInstances == 0 {
		return 1
	}
	return numInstances
}

func serviceEnv(runtime client.ServiceRuntime) client.ServiceEnv {
	return client.ServiceEnv(runtime)
}

func dockerEnvDetails(attrs DockerRuntimeAttrs) client.EnvSpecificDetails {
	dockerCommand := attrs.DockerCommand
	if dockerCommand == "" {
		dockerCommand = "bin/start"
	}
	dockerContext := attrs.DockerContext
	if dockerContext == "" {
		dockerContext = "."
	}
	dockerfilePath := attrs.DockerfilePath
	if dockerfilePath == "" {
		dockerfilePath = "./Dockerfile"
	}
	var details client.EnvSpecificDetails
	must(details.FromDockerDetails(client.DockerDetails{
		DockerCommand:      dockerCommand,
		DockerContext:      dockerContext,
		DockerfilePath:     dockerfilePath,
		RegistryCredential: attrs.RegistryCredential,
	}))
	return details
}

// nativeCommandDefaults returns the default build command and start command,
// in that order, for a native runtime.
func nativeCommandDefaults(runtime client.ServiceRuntime) (string, string) {
	switch runtime {
	case client.ServiceRuntimeElixir:
		return "mix deps.get && mix compile", "mix phx.server"
	case client.ServiceRuntimeNode:
		return "npm install", "npm start"
	case client.ServiceRuntimePython:
		return "pip install -r requirements.txt", "python app.py"
	case client.ServiceRuntimeRuby:
		return "bundle install", "bundle exec puma"
	case client.ServiceRuntimeRust:
		return "cargo build --release", "./target/release/app"
	default:
		return "go build ./...", "./render"
	}
}

func nativeEnvDetails(runtime client.ServiceRuntime, buildCommand, startCommand string) client.EnvSpecificDetails {
	defaultBuildCommand, defaultStartCommand := nativeCommandDefaults(runtime)
	if buildCommand == "" {
		buildCommand = defaultBuildCommand
	}
	if startCommand == "" {
		startCommand = defaultStartCommand
	}
	var details client.EnvSpecificDetails
	must(details.FromNativeEnvironmentDetails(client.NativeEnvironmentDetails{
		BuildCommand: buildCommand,
		StartCommand: startCommand,
	}))
	return details
}

func webServiceDetails(attrs WebServiceDetailsAttrs, runtime runtimeDetails) client.WebServiceDetails {
	healthCheckPath := attrs.HealthCheckPath
	if healthCheckPath == "" {
		healthCheckPath = "/healthz"
	}
	url := attrs.URL
	if url == "" {
		url = "https://example.onrender.com"
	}
	return client.WebServiceDetails{
		BuildPlan:          client.BuildPlanStarter,
		Env:                serviceEnv(runtime.runtime),
		EnvSpecificDetails: runtime.envSpecificDetails,
		HealthCheckPath:    healthCheckPath,
		NumInstances:       numInstancesOrDefault(attrs.NumInstances),
		OpenPorts:          []client.ServerPort{},
		Plan:               planOrDefault(attrs.Plan),
		Region:             regionOrDefault(attrs.Region),
		Runtime:            runtime.runtime,
		Url:                url,
	}
}

func backgroundWorkerDetails(attrs BackgroundWorkerDetailsAttrs, runtime runtimeDetails) client.BackgroundWorkerDetails {
	return client.BackgroundWorkerDetails{
		BuildPlan:          client.BuildPlanStarter,
		Env:                serviceEnv(runtime.runtime),
		EnvSpecificDetails: runtime.envSpecificDetails,
		NumInstances:       numInstancesOrDefault(attrs.NumInstances),
		Plan:               planOrDefault(attrs.Plan),
		Region:             regionOrDefault(attrs.Region),
		Runtime:            runtime.runtime,
	}
}

func privateServiceDetails(attrs PrivateServiceDetailsAttrs, runtime runtimeDetails) client.PrivateServiceDetails {
	url := attrs.URL
	if url == "" {
		url = "private.internal"
	}
	return client.PrivateServiceDetails{
		BuildPlan:          client.BuildPlanStarter,
		Env:                serviceEnv(runtime.runtime),
		EnvSpecificDetails: runtime.envSpecificDetails,
		NumInstances:       numInstancesOrDefault(attrs.NumInstances),
		OpenPorts:          []client.ServerPort{},
		Plan:               planOrDefault(attrs.Plan),
		Region:             regionOrDefault(attrs.Region),
		Runtime:            runtime.runtime,
		Url:                url,
	}
}

func staticSiteDetails(attrs StaticSiteDetailsAttrs) client.StaticSiteDetails {
	buildCommand := attrs.BuildCommand
	if buildCommand == "" {
		buildCommand = "npm run build"
	}
	publishPath := attrs.PublishPath
	if publishPath == "" {
		publishPath = "dist"
	}
	url := attrs.URL
	if url == "" {
		url = "https://example.onrender.com"
	}
	return client.StaticSiteDetails{
		BuildCommand: buildCommand,
		BuildPlan:    client.BuildPlanStarter,
		PublishPath:  publishPath,
		Url:          url,
	}
}

func cronJobDetails(attrs CronJobDetailsAttrs, runtime runtimeDetails) client.CronJobDetails {
	schedule := attrs.Schedule
	if schedule == "" {
		schedule = "0 0 * * *"
	}
	return client.CronJobDetails{
		BuildPlan:          client.BuildPlanStarter,
		Env:                serviceEnv(runtime.runtime),
		EnvSpecificDetails: runtime.envSpecificDetails,
		Plan:               planOrDefault(attrs.Plan),
		Region:             regionOrDefault(attrs.Region),
		Runtime:            runtime.runtime,
		Schedule:           schedule,
	}
}

func (s *Server) serviceInstances() []*client.Service {
	services := make([]*client.Service, len(s.Services.Instances))
	copy(services, s.Services.Instances)
	return services
}

func registerServiceRoutes(mux *http.ServeMux, s *Server, record func(*http.Request)) {
	// GET /services - list services (supports ?name=, ?type=, ?ownerId=, and ?environmentId= filters)
	mux.HandleFunc("GET /services", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if status, hasError := s.Services.nextError(); hasError {
			w.WriteHeader(status)
			return
		}
		name := r.URL.Query().Get("name")
		serviceTypes := queryListValues(r, "type")
		ownerIDs := queryListValues(r, "ownerId")
		envIDs := queryListValues(r, "environmentId")
		services := s.serviceInstances()
		result := make([]client.ServiceWithCursor, 0, len(services))
		for i, svc := range services {
			if name != "" && svc.Name != name {
				continue
			}
			if len(serviceTypes) > 0 && !slices.Contains(serviceTypes, string(svc.Type)) {
				continue
			}
			if len(ownerIDs) > 0 && !slices.Contains(ownerIDs, svc.OwnerId) {
				continue
			}
			if len(envIDs) > 0 {
				if svc.EnvironmentId == nil || !slices.Contains(envIDs, *svc.EnvironmentId) {
					continue
				}
			}
			result = append(result, client.ServiceWithCursor{
				Cursor:  client.Cursor(fmt.Sprintf("c%d", i)),
				Service: *svc,
			})
		}
		writeJSON(w, http.StatusOK, result)
	})

	// GET /services/{id} - retrieve a service
	mux.HandleFunc("GET /services/{id}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if status, hasError := s.Services.nextError(); hasError {
			w.WriteHeader(status)
			return
		}
		id := r.PathValue("id")
		for _, svc := range s.serviceInstances() {
			if svc.Id == id {
				writeJSON(w, http.StatusOK, svc)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// PATCH /services/{id} - update a service
	mux.HandleFunc("PATCH /services/{id}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if status, hasError := s.Services.nextError(); hasError {
			w.WriteHeader(status)
			return
		}

		var body client.UpdateServiceJSONRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		idx := slices.IndexFunc(s.Services.Instances, func(svc *client.Service) bool {
			return svc.Id == r.PathValue("id")
		})
		if idx == -1 {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		svc := s.Services.Instances[idx]
		if body.Name != nil {
			svc.Name = *body.Name
		}
		if body.Branch != nil {
			svc.Branch = body.Branch
		}
		svc.UpdatedAt = time.Now()
		writeJSON(w, http.StatusOK, svc)
	})

	// DELETE /services/{id} - delete a service
	mux.HandleFunc("DELETE /services/{id}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if status, hasError := s.Services.nextError(); hasError {
			w.WriteHeader(status)
			return
		}
		id := r.PathValue("id")
		for i, svc := range s.Services.Instances {
			if svc.Id == id {
				s.Services.Instances = slices.Delete(s.Services.Instances, i, i+1)
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})
}
