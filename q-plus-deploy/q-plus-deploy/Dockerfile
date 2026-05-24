## Build
FROM golang:1.22-alpine AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

RUN go build -o /q_plus

## Deploy
FROM gcr.io/distroless/base-debian11

WORKDIR /

COPY --from=build /q_plus /q_plus
COPY --from=build /app/google_credentials.json /google_credentials.json

#EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/q_plus"]
