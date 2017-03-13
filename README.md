# Stardog Service Broker

This repository implements a Stardog Service Broker for use with [Cloud 
Foundry](https://www.cloudfoundry.org/).  It implements the [Open
Services Broker API](https://www.openservicebrokerapi.org/) and thus
should be compatible with other cloud native platforms like 
[OpenShift](https://www.openshift.org/) and
[Kubernetes](http://kubernetes.io/).  At this time it has only been
tested against Cloud Foundry.

# Getting Started

In order to use this service broker a running instance of
[Stardog](http://stardog.com/) and administrator access to it is
required.  While this broker has been designed to run inside of Cloud 
Foundry it is not required.

## Configuration

A configuration file is required to run the broker.  A sample
configuration can be found [here](data/conf.json.example).  It is a
JSON document with the following fields.

### Configuration Objects

| Field             | Type      | Description
| -----             | ----      | ------------ |
| broker_username*  | string    | All requests to this service broker must include HTTP basic authentication headers with this username. |
| broker_password*  | string    | All requests to this service broker must include HTTP basic authentication headers with this password. |
| broker_id*        | string    | The port on which the service broker will listen for HTTP connections. |
| port              | string    | The level at which the broker will log.  Values can be ERROR, WARN, INFO, and DEBUG.  INFO is the default. |
| log_level         | string    | The level at which the broker will log.  Values can be ERROR, WARN, INFO, and DEBUG.  INFO is the default. |
| log_file          | string    | A path to a file while log lines will be stored.  The default is stderr. |
| plans             | array of plan-descriptor | The list of plans that this service will offer. |
| storage           | storage-descriptor*      | The storage module that will be used to persist data relevant service broker data. | 

#### plan-descriptor

| Field             | Type      | Description
| -----             | ----      | ------------ |
| name*             | string    | The name of the plan. |
| ID*               | string    | The ID of the plan.  This must be globally unique. |
| Parameters        | JSON      | A JSON document which is defined by the specific plan defined in this block. |

#### storage-descriptor

| Field             | Type      | Description
| -----             | ----      | ------------ |
| type*             | string    | The type of storage driver to use.  Currently "stardog" is the only valid value. |
| parameters        | JSON      | A JSON document which is defined by the specific storage driver. |

#### Plans

Currently the only plan implemented is the shared_database_plan.  This
plan uses a single Stardog service.  When Cloud Foundry requests a new
service instance from the broker this plan will create a new database.
When Cloud Foundry then requests that an application be bound to a
service instance this plan will create a new user and grant it access
to the associated database.

| Field             | Type      | Description
| -----             | ----      | ------------ |
| stardog_url*      | string    | The URL to the Stardog service that will be used by the plan to create databases. |
| admin_username*   | string    | The administrator user name for the Stardog service. |
| admin_password*   | string    | The administrator password for the Stardog service. |

#### Storage Drivers

Currently the only valid storage driver is *stardog*.  This driver uses
a Stardog database for storing the metadata that the service broker
needs to be effective.  It can be the same Stardog server that is used
with a plan.

| Field             | Type      | Description
| -----             | ----      | ------------ |
| stardog_url*      | string    | The URL to the Stardog service that will be used by the plan to create databases. |
| admin_username*   | string    | The administrator user name for the Stardog service. |
| admin_password*   | string    | The administrator password for the Stardog service. |

## Running

### In Cloud Foundry

To run inside of Cloud Foundry create the configuration file
data/conf.json according to the documentation above.  The run the
`cf push` as shown in the sample session below.

```
 cf push
Using manifest file /Users/bresnaha/go/src/github.com/stardog-union/service-broker/manifest.yml

Creating app stardog-service-broker in org stardog.com / space spacedog as admin...
OK

Using route stardog-service-broker.bosh-lite.com
Binding stardog-service-broker.bosh-lite.com to stardog-service-broker...
OK

Uploading stardog-service-broker...
Uploading app files from: /Users/bresnaha/go/src/github.com/stardog-union/service-broker
Uploading 99.2K, 78 files
Done uploading
OK

Starting app stardog-service-broker in org stardog.com / space spacedog as admin...
-----> Downloaded app package (4.8M)
Cloning into '/tmp/buildpacks/go-buildpack'...
Submodule 'compile-extensions' (https://github.com/cloudfoundry/compile-extensions.git) registered for path 'compile-extensions'
Cloning into 'compile-extensions'...
Submodule path 'compile-extensions': checked out 'c335327df3f71b8fca7985be30ed75f828af6bac'
-------> Buildpack version 1.7.19
https://buildpacks.cloudfoundry.org/dependencies/godep/godep-v79-linux-x64-9e37ce0f.tgz
https://buildpacks.cloudfoundry.org/dependencies/glide/glide-v0.12.3-linux-x64-5b2e71ff.tgz
-----> Checking Godeps/Godeps.json file.
-----> Installing go1.7.5... done
Downloaded [https://buildpacks.cloudfoundry.org/dependencies/go/go1.7.5.linux-amd64-c9de5bb9.tar.gz]
 !!    Installing package '.' (default)
-----> Running: go install -v -tags cloudfoundry .
github.com/stardog-union/service-broker/vendor/github.com/gorilla/mux
github.com/stardog-union/service-broker/broker
github.com/stardog-union/service-broker/plans/shared
github.com/stardog-union/service-broker
-----> Uploading droplet (6.9M)

1 of 1 instances running

App started


OK

App stardog-service-broker was started using this command `service-broker data/conf.json`

Showing health and status for app stardog-service-broker in org stardog.com / space spacedog as admin...
OK

requested state: started
instances: 1/1
usage: 512M x 1 instances
urls: stardog-service-broker.bosh-lite.com
last uploaded: Mon Mar 13 18:18:47 UTC 2017
stack: cflinuxfs2
buildpack: https://github.com/cloudfoundry/go-buildpack.git

     state     since                    cpu    memory         disk      details
#0   running   2017-03-13 08:19:41 AM   0.0%   8.5M of 512M   0 of 1G
```

### Outside of Cloud Foundry

To run the broker outside of cloud foundry do the following steps:

1. Create the configuration file described above.
2. Compile the program.  A proper Go development environment must be
   setup to complete this step.  Make sure that GOPATH is pointing to
   the root of you Go development environment and that this archive
   is checked out under `$GOPATH/src/github.com/stardog-union/service-broker`.
   Then run the script [./bin/build.sh](./bin/build.sh).
3. Run the program.  The service broker will be compiled to
   `./stardog-service-broker`.  Run it by passing it the path to the
   configuration file as the only input.
   
   ```
   $ ./stardog-service-broker data/conf.json
   USING PORT: 8080
   ```

# VCAP_SERVICES Definition

When an application is bound to a service instance in Cloud Foundry
the environment variable VCAP_SERVICES is defined with information
about the services to which it had been bound.  More information about
this can be found [here](http://docs.cloudfoundry.org/devguide/deploy-apps/environment-variable.html#VCAP-SERVICES).

When using the *shared_database_plan* the VCAP_SERVICES document will
have the following credentials definition under its plan.

| Field             | Type      | Description
| -----             | ----      | ------------ |
| url*              | string    | The URL of the Stardog service that is bound to the application. |
| db_name*          | string    | The Stardog database name to which the application is bound . |
| username*         | string    | The username credential that the application can use to connect to the stardog database. |
| password*         | string    | The password credential that the application can use to connect to the stardog database. |


Below is an example VCAP_SERVICES document.

```
{"Stardog":
   [
     {"name": "stardog-service",
      "label": "Stardog",
      "tags": [],
      "plan": "shareddb",
      "credentials": {
          "db_name": "dbABzeMjTwwwvyuoiO",
          "url": "http://somehost.com:5821",
          "password": "QQttqbmLVkzvfaWVKlUrthnT",
          "username": "stardogHOPvOMwo"
      }
     }
   ]
}
```
