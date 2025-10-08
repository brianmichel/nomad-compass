# syntax=docker/dockerfile:1

FROM node:20 AS frontend
WORKDIR /workspace/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

FROM golang:1.24 AS backend
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /workspace/internal/web/dist internal/web/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/nomad-compass ./cmd/nomad-compass

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=backend /workspace/bin/nomad-compass /app/nomad-compass
COPY --from=backend /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
EXPOSE 8080
ENTRYPOINT ["/app/nomad-compass"]
