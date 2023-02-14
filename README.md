# Waypoint Plugin Seaplane

This folder contains the Waypoint-Seaplane plugin used to deploy your compute workloads on Seaplane through Waypoint. You will need to have Waypoint installed and configured on your machine. You can learn more about Waypoint [here](https://developer.hashicorp.com/waypoint/tutorials/get-started-docker/get-started-install).

You can build the integration yourself by running `make && make install` in the root directory of the integration. Copy the compiled executable to your project root directory or the waypoint plugin directory.


# Getting started 

To get started create a `waypoint.hcl` file in your project's root directory with the following components. To learn more about flights, formations and other Seaplane terminology have a look at our documentation [here](https://developers.seaplane.io/docs/compute/terminology/compute-flights).

```
project = "project-name"

app "app" {
  build {
    use "docker" {}
    registry {
      use "docker" {
        image = "registry.cplane.cloud/${var.seaplane_tenant}/hashitalks"
        tag   = var.tag
      }
    }
  }

  deploy {
    use "seaplane" {

      formation_name = "your-formation-name"
      flight_name = "your-flight-name"

      api_key = "my-super-secret-api-key"

    }
  }
}

variable "seaplane_tenant" {
  type        = string
  description = "The tag for the built image in the Docker registry."
}

variable "tag" {
  default     = "latest"
  type        = string
  description = "The tag for the built image in the Docker registry."
}
```

Replace the `project-name`, `your-formation-name`, `your-flight-name` and `my-super-secret-api-key` with your values. Make sure you are logged in to the Seaplane container registry, before moving forward. You can learn more about loging in [here](https://developers.seaplane.io/docs/compute/registry/authentication). 

with your `waypoint.hcl` file in place run `waypoint init` in your project directory. Followed by `waypoint up`. Waypoint will run through the build, push and deploy step and once completed share a resource URL with you that points to the deployed container. This is a single resource URL that depending on your location always routes to the closest deployment. To learn more about how Seaplane automates DR, routing, scaling and much more read our documentation [here](https://developers.seaplane.io/docs/compute/compute-intro).

To take down your deployment run `waypoint destroy` in your project root directory.
