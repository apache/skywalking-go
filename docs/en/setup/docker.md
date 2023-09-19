# Setup in docker

SkyWalking Go supports building user applications using Docker as the base container image.

## Customized Dockerfile

Using the SkyWalking Go provided image as the base image, perform file copying and other operations in the Dockerfile.

```dockerfile
# import the skywalking go base image
FROM apache/skywalking-go:<version>-go<go version>

# Copy application code
COPY /path/to/project /path/to/project
# Inject the agent into the project or get dependencies by application self
RUN skywalking-go-agent -inject /path/to/project
# Building the project including the agent
RUN go build -toolexec="skywalking-go-agent" -a /path/to/project

# More operations
...
```

In the above code, we have performed the following actions:

1. Used the SkyWalking Go provided image as the base image, which currently supports the following Go versions: **1.16, 1.17, 1.18, 1.19, 1.20**.
2. Copied the project into the Docker image.
3. Installed SkyWalking Go and compiled the project, [read this documentation for more detail](./gobuild.md). 
The SkyWalking Go agent is already installed in the `/usr/local/bin` directory with the name **skywalking-go-agent**.
