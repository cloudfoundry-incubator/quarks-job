ARG BASE_IMAGE=registry.opensuse.org/cloud/platform/quarks/sle_15_sp1/quarks-operator-base:latest

FROM golang:1.14.7 AS build
ARG GOPROXY
ENV GOPROXY $GOPROXY

WORKDIR /go/src/code.cloudfoundry.org/quarks-job

# Copy the rest of the source code and build
COPY . .
RUN bin/build && \
    cp -p binaries/quarks-job /usr/local/bin/quarks-job

FROM $BASE_IMAGE
RUN groupadd -g 1000 quarks && \
    useradd -r -u 1000 -g quarks quarks
USER quarks
COPY --from=build /usr/local/bin/quarks-job /usr/local/bin/quarks-job
ENTRYPOINT ["/tini", "--", "/usr/local/bin/quarks-job"]
