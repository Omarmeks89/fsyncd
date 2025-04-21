FROM golang:1.23.7-alpine3.21 AS build-stage

WORKDIR     /fsyncd
COPY        go.*            ./
RUN go mod download

COPY *.go                   ./

# create and set user
RUN apk add --no-cache shadow
RUN apk add --no-cache tzdata
RUN addgroup -S fsyncd && \
    adduser -S -D -H -G fsyncd fsyncd

# compile
RUN CGO_ENABLED=0 go build -trimpath -o /fsyncd

FROM build-stage AS bs

# set owner
RUN chown fsyncd:fsyncd /fsyncd
USER fsyncd

FROM scratch
COPY --from=build-stage /fsyncd /fsyncd
COPY --from=build-stage /usr    /usr
COPY --from=build-stage /etc    /etc
