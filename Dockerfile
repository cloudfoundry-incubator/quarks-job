FROM golang:1.13 AS build
ARG GOPROXY
ENV GOPROXY $GOPROXY
ARG GO111MODULE="on"
ENV GO111MODULE $GO111MODULE

WORKDIR /go/src/code.cloudfoundry.org/quarks-job
# First, download dependencies so we can cache this layer
COPY go.mod .
COPY go.sum .
RUN if [ "${GO111MODULE}" = "on" ]; then go mod download; fi

# Copy the rest of the source code and build
COPY . .
RUN bin/build && \
    cp -p binaries/quarks-job /usr/local/bin/quarks-job

FROM cfcontainerization/cf-operator-base
RUN groupadd -g 1000 quarks && \
    useradd -r -u 1000 -g quarks quarks
USER quarks
COPY --from=build /usr/local/bin/quarks-job /usr/local/bin/quarks-job
ENTRYPOINT ["/tini", "--"]
CMD ["/usr/local/bin/quarks-job"]
