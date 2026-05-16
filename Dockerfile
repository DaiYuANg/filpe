FROM golang:1.26-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/maxio ./cmd/maxio

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=build /out/maxio /app/maxio
COPY config.example.json /app/config.json

VOLUME ["/app/data"]
EXPOSE 8080 63000 7946

ENTRYPOINT ["/app/maxio"]
