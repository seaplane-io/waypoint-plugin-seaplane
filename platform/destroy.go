package platform

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/hashicorp/go-hclog"
	sdk "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
	"github.com/hashicorp/waypoint-plugin-sdk/terminal"
)

// Implement the Destroyer interface
func (p *Platform) DestroyFunc() interface{} {
	return p.destroy
}

// A DestroyFunc does not have a strict signature, you can define the parameters
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
//
// In addition to default input parameters the Deployment from the DeployFunc step
// can also be injected.
//
// The output parameters for PushFunc must be a Struct which can
// be serialzied to Protocol Buffers binary format and an error.
// This Output Value will be made available for other functions
// as an input parameter.
//
// If an error is returned, Waypoint stops the execution flow and
// returns an error to the user.
func (p *Platform) destroy(
	ctx context.Context,
	ui terminal.UI,
	log hclog.Logger,
	deployment *Deployment,
) error {
	sg := ui.StepGroup()
	defer sg.Wait()
	// u := ui.Status()

	rm := p.resourceManager(log, nil)

	// If we don't have resource state, this state is from an older version
	// and we need to manually recreate it.
	if deployment.ResourceState == nil {
		rm.Resource("deployment").SetState(&Resource_Deployment{
			Name: deployment.Name,
		})
	} else {
		// Load our set state
		if err := rm.LoadState(deployment.ResourceState); err != nil {
			return err
		}
	}

	// Destroy your deployment resource
	u := ui.Status()

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
	req.Header.Add("Authorization", "Bearer "+p.config.APIKey)
	req.Header.Add("Content-Length", "0")

	// perform request
	resp, err := client.Do(req)

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

	// Destroy current formation based on stanza
	req, err = http.NewRequest("DELETE", "https://compute.cplane.cloud/v1/formations/"+p.config.FormationName+"?force=true", nil)

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
		u.Step(terminal.ErrorStyle, fmt.Sprintf("Unable to remove formation %s", p.config.FormationName))
		return err
	}

	// close the response stream
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		u.Step(terminal.StatusOK, fmt.Sprintf("Removed formation %s", p.config.FormationName))

		// update status
		rm.StatusReport(sdk.StatusReport_DOWN)
	}

	defer u.Close()

	return rm.DestroyAll(ctx, log, sg, ui)

}

func (b *Platform) resourceDeploymentDestroy(
	ctx context.Context,
	log hclog.Logger,
	sg terminal.StepGroup,
	ui terminal.UI,
) error {

	// // Destroy your deployment resource
	// u := ui.Status()

	// // create http client
	// client := &http.Client{}

	// // Authenticate with seaplane
	// u.Update("Authenticating with Seaplane")

	// // create token request
	// req, err := http.NewRequest("POST", "https://flightdeck.cplane.cloud/identity/token", nil)

	// // handle error
	// if err != nil {
	// 	panic(err)
	// }

	// // add request headers with API key and set content length header to 0
	// req.Header.Add("Authorization", "Bearer "+b.config.APIKey)
	// req.Header.Add("Content-Length", "0")

	// // perform request
	// resp, err := client.Do(req)

	// // handle errors on get token request
	// if err != nil {
	// 	panic(err)
	// }

	// // close the response stream
	// defer resp.Body.Close()

	// // save the token
	// token, _ := ioutil.ReadAll(resp.Body)

	// // create bearer string
	// bearer := "Bearer " + string(token)

	// // Destroy current formation based on stanza
	// req, err = http.NewRequest("DELETE", "https://compute.cplane.cloud/v1/formations/"+b.config.FormationName+"?force=true", nil)

	// // handle error
	// if err != nil {
	// 	panic(err)
	// }

	// // add authentication header
	// req.Header.Add("Authorization", bearer)

	// // create the formation
	// resp, err = client.Do(req)

	// // handle errors on get token request
	// if err != nil {
	// 	u.Step(terminal.ErrorStyle, fmt.Sprintf("Unable to remove formation %s", b.config.FormationName))
	// 	return err
	// }

	// // close the response stream
	// defer resp.Body.Close()

	// if resp.StatusCode == 200 {
	// 	u.Step(terminal.StatusOK, fmt.Sprintf("Removed formation %s", b.config.FormationName))
	// }

	// defer u.Close()

	return nil
}
