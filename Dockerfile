FROM golang:1.19-alpine3.16 as build

# Install the Protocol Buffers compiler and Go plugin
RUN apk add protobuf git make zip
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Create the source folder
RUN mkdir /go/plugin
WORKDIR /go/plugin

# Copy the source to the build folder
COPY . /go/plugin

# Build the plugin
RUN chmod +x ./print_arch
RUN make all

# Create the zipped binaries
RUN make zip

FROM scratch as export_stage

COPY --from=build /go/plugin/bin/*.zip .
