# syntax=docker/dockerfile:1

FROM node:20 AS frontend
WORKDIR /workspace/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.24 AS backend
ARG TARGETOS
ARG TARGETARCH
ENV CGO_ENABLED=0
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /workspace/internal/web/dist internal/web/dist
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o bin/nomad-compass ./cmd/nomad-compass

FROM --platform=$TARGETPLATFORM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=backend /workspace/bin/nomad-compass /app/nomad-compass
COPY --from=backend /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
EXPOSE 8080
ENTRYPOINT ["/app/nomad-compass"]
