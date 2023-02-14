package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	// httputil can go only used for debugging

	"regexp"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/waypoint-plugin-sdk/component"
	"github.com/hashicorp/waypoint-plugin-sdk/framework/resource"
	sdk "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
	"github.com/hashicorp/waypoint-plugin-sdk/terminal"
	"github.com/hashicorp/waypoint/builtin/docker"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// adding URL to deployment
func (d *Deployment) URL() string {
	return d.Url
}

// HCL input variables
type DeployConfig struct {
	FormationName string `hcl:"formation_name"`
	FlightName    string `hcl:"flight_name"`
	APIKey        string `hcl:"api_key"`
}

type Platform struct {
	config DeployConfig
}

// flight struct
type Flights struct {
	Name    string `json:"name"`
	Image   string `json:"image"`
	Minimum int    `json:"minimum"`
	Maximum int    `json:"maximum"`
}

// formation struct
type Formation struct {
	Flights []Flights `json:"flights"`
}

// url struct for the launched formation
type FormationURL struct {
	FormationUrl string `json:"url"`
}

// Implement Configurable
func (p *Platform) Config() (interface{}, error) {
	return &p.config, nil
}

// Implement ConfigurableNotify
func (p *Platform) ConfigSet(config interface{}) error {
	c, ok := config.(*DeployConfig)
	if !ok {
		// The Waypoint SDK should ensure this never gets hit
		return fmt.Errorf("expected *DeployConfig as parameter")

	}

	// validate the config

	// max length formation name is 27
	if len(c.FormationName) > 27 {
		return fmt.Errorf("formation name can not be longer than 27 characters")
	}

	// formation name only allows for alphanumeric characters
	if regexp.MustCompile(`^[a-zA-Z0-9]*$`).MatchString(c.FormationName) {
		return fmt.Errorf("formation names can only contain [a-z] [A-Z] and hyphens")
	}

	// no double hypens in formation name
	if strings.Contains(c.FormationName, "--") {
		return fmt.Errorf("formation names can not contain double hyphens --")
	}

	// check flight name length
	if len(c.FlightName) > 27 {
		return fmt.Errorf("flight name can not be longer than 27 characters")
	}

	// check formation name only alphanumeric characters
	if regexp.MustCompile(`^[a-zA-Z0-9]*$`).MatchString(c.FormationName) {
		return fmt.Errorf("flight names can only contain [a-z] [A-Z] and hyphens")
	}

	// No double hyphens in flight name
	if strings.Contains(c.FormationName, "--") {
		return fmt.Errorf("flight names can not contain double hyphens --")
	}

	return nil
}

// This function can be implemented to return various connection info required
// to connect to your given platform for Resource Manager. It could return
// a struct with client information, what namespace to connect to, a config,
// and so on.
func (p *Platform) getConnectContext() (interface{}, error) {
	return nil, nil
}

// Resource manager will tell the Waypoint Plugin SDK how to create and delete
// certain resources for your deployments.
//
// For example, your deployment might need to create a "container" or "load balancer".
// Your plugin could implement two resources through ResourceManager and the Waypoint
// Plugin SDK will automatically create or delete these resources as well as
// obtain the defined status for them.
//
// ResourceManager can also be implemented for Release as well.
func (p *Platform) resourceManager(log hclog.Logger, dcr *component.DeclaredResourcesResp) *resource.Manager {
	return resource.NewManager(
		resource.WithLogger(log.Named("resource_manager")),
		resource.WithValueProvider(p.getConnectContext),
		resource.WithDeclaredResourcesResp(dcr),
		resource.WithResource(resource.NewResource(
			resource.WithName("template_example"),
			resource.WithState(&Resource_Deployment{}),
			resource.WithCreate(p.resourceDeploymentCreate),
			resource.WithDestroy(p.resourceDeploymentDestroy),
			resource.WithStatus(p.resourceDeploymentStatus),
			resource.WithPlatform("template_platform"),                                         // Update this to match your plugins platform, like Kubernetes
			resource.WithCategoryDisplayHint(sdk.ResourceCategoryDisplayHint_INSTANCE_MANAGER), // This is meant for the UI to determine what kind of icon to show
		)),
		// NOTE: Add more resource funcs here if your plugin has more than 1 resource
	)
}

// Implement Builder
func (p *Platform) DeployFunc() interface{} {
	// return a function which will be called by Waypoint
	return p.deploy
}

func (p *Platform) StatusFunc() interface{} {
	return p.status
}

// A BuildFunc does not have a strict signature, you can define the parameters
// you need based on the Available parameters that the Waypoint SDK provides.
// Waypoint will automatically inject parameters as specified
// in the signature at run time.
//
// Available input parameters:
// - context.Context
// - *component.Source
// - *component.JobInfo
// - *component.DeploymentConfig
// - hclog.Logger
// - terminal.UI
// - *component.LabelSet

// In addition to default input parameters the registry.Artifact from the Build step
// can also be injected.
//
// The output parameters for BuildFunc must be a Struct which can
// be serialzied to Protocol Buffers binary format and an error.
// This Output Value will be made available for other functions
// as an input parameter.
// If an error is returned, Waypoint stops the execution flow and
// returns an error to the user.
func (b *Platform) deploy(
	ctx context.Context,
	ui terminal.UI,
	log hclog.Logger,
	dcr *component.DeclaredResourcesResp,
	img *docker.Image,

	// artifact *registry.Artifact,
) (*Deployment, error) {
	u := ui.Status()
	defer u.Close()
	u.Update("Deploy application")

	var result Deployment

	// Create our resource manager and create deployment resources
	rm := b.resourceManager(log, dcr)

	// These params must match exactly to your resource manager functions. Otherwise
	// they will not be invoked during CreateAll()
	if err := rm.CreateAll(
		ctx, log, u, ui,
		// artifact, &result,
		&result,
	); err != nil {
		return nil, err
	}

	// update user
	u.Update("Deploying Application on Seaplane")

	// create http client
	client := &http.Client{}

	// Authenticate with seaplane
	u.Update("Authenticating with Seaplane")

	// create token request
	req, err := http.NewRequest("POST", "https://flightdeck.cplane.cloud/identity/token", nil)

	// handle error
	if err != nil {
		panic(err)
	}

	// add request headers with API key and set content length header to 0
	req.Header.Add("Authorization", "Bearer "+b.config.APIKey)
	req.Header.Add("Content-Length", "0")

	// perform request
	resp, err := client.Do(req)

	if resp.StatusCode == 200 {
		u.Step(terminal.StatusOK, "Successfully authenticated with Seaplane")
	}

	// handle errors on get token request
	if err != nil {
		panic(err)
	}

	// close the response stream
	defer resp.Body.Close()

	// save the token
	token, _ := ioutil.ReadAll(resp.Body)

	// create bearer string
	bearer := "Bearer " + string(token)

	// get the image URL and architecture from the build step
	img_url := img.Name()
	// img_arch := img.architecture

	// construct the flight based on the input variables
	var flights = []Flights{
		{
			Name:    b.config.FlightName,
			Image:   img_url,
			Minimum: 1,
			Maximum: 1,
		},
	}

	// constrcut the formation payload
	formation := Formation{
		Flights: flights,
	}

	// update user
	u.Update("Launching Formation")

	//Convert formation to byte using Json.Marshal
	body, _ := json.Marshal(formation)

	// create post request to create formation
	req, err = http.NewRequest("POST", "https://compute.cplane.cloud/v1/formations/"+b.config.FormationName, bytes.NewBuffer(body))

	// handle error
	if err != nil {
		panic(err)
	}

	// add authentication header
	req.Header.Add("Authorization", bearer)

	// create the formation
	resp, err = client.Do(req)

	// handle errors on get token request
	if err != nil {
		panic(err)
	}

	// close the response stream
	defer resp.Body.Close()

	// show the user the result of our deployment
	if resp.StatusCode == 200 {
		u.Step(terminal.StatusOK, "Application deployed successfully")
		rm.StatusReport(sdk.StatusReport_ALIVE)

		// set formaton name of the deployed application to be used in destroy func.
		result.Name = b.config.FormationName

		// get the URL for our formation
		req, err = http.NewRequest("GET", "https://compute.cplane.cloud/v1/formations/"+b.config.FormationName, nil)

		// handle error
		if err != nil {
			panic(err)
		}

		// add authentication header
		req.Header.Add("Authorization", bearer)

		// create the formation
		resp, err = client.Do(req)

		// handle errors on get token request
		if err != nil {
			panic(err)
		}

		// read the result
		body, _ := ioutil.ReadAll(resp.Body)

		var formationURL FormationURL
		if err := json.Unmarshal(body, &formationURL); err != nil { // Parse []byte to go struct pointer
			fmt.Println("Can not unmarshal JSON")
		}

		// close the response stream
		defer resp.Body.Close()

		// update user with the URL
		u.Step(terminal.StatusOK, "We are launching your formation on "+formationURL.FormationUrl+" give it a minute if the URL does not immediately load")

		// result.Url = string(body)

	} else if resp.StatusCode == 400 {
		u.Step(terminal.StatusError, "There was something wrong with your request (this is usually caused by an issue with your included configuration)")
	} else if resp.StatusCode == 401 {
		u.Step(terminal.StatusError, "You are not logged in (try setting the `Authorization` header)")
	} else if resp.StatusCode == 403 {
		u.Step(terminal.StatusError, "You have insufficient permissions to perform this action")
	} else if resp.StatusCode == 404 {
		u.Step(terminal.StatusError, "The source for the clone operation was not found")
	} else if resp.StatusCode == 409 {
		u.Step(terminal.StatusError, "There is already a formation with this name, formation names must be unique within your organization")
	}

	// Store our resource state
	result.ResourceState = rm.State()

	return &result, nil

}

// This function is the top level status command that gets invoked when Waypoint
// attempts to determine the health of a dpeloyment. It will also invoke the
// status for each resource involed for the given deployment if any.
func (d *Platform) status(
	ctx context.Context,
	ji *component.JobInfo,
	ui terminal.UI,
	log hclog.Logger,
	deployment *Deployment,
) (*sdk.StatusReport, error) {
	// sg := ui.StepGroup()
	// s := sg.Add("Checking the status of the deployment...")

	// rm := d.resourceManager(log, nil)

	// // If we don't have resource state, this state is from an older version
	// // and we need to manually recreate it.
	// if deployment.ResourceState == nil {
	// 	rm.Resource("deployment").SetState(&Resource_Deployment{
	// 		Name: deployment.Id,
	// 	})
	// } else {
	// 	// Load our set state
	// 	if err := rm.LoadState(deployment.ResourceState); err != nil {
	// 		return nil, err
	// 	}
	// }

	// // This will call the StatusReport func on every defined resource in ResourceManager
	// report, err := rm.StatusReport(ctx, log, sg, ui)
	// if err != nil {
	// 	return nil, status.Errorf(codes.Internal, "resource manager failed to generate resource statuses: %s", err)
	// }

	// // report.Health = sdk.StatusReport_UNKNOWN
	// report.Health = sdk.StatusReport_ALIVE
	// s.Update("Deployment is currently not implemented!")
	// s.Done()

	// Create our status report
	report := &sdk.StatusReport{}
	report.External = true
	report.Health = sdk.StatusReport_READY
	report.HealthMessage = "Application successfully deployed"
	report.GeneratedTime = timestamppb.Now()

	return report, nil
}

func (b *Platform) resourceDeploymentCreate(
	ctx context.Context,
	log hclog.Logger,
	st terminal.Status,
	ui terminal.UI,
	// artifact *registry.Artifact,
	result *Deployment,
) error {

	return nil
}

func (b *Platform) resourceDeploymentStatus(
	ctx context.Context,
	ui terminal.UI,
	sg terminal.StepGroup,
	// artifact *registry.Artifact,
) error {
	// Determine health status of "this" resource.
	return nil
}
