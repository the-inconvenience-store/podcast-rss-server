FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/podcast-rss ./cmd/podcast-rss

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/podcast-rss /app/podcast-rss
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/podcast-rss"]
