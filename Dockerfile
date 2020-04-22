FROM golang:1.14.2-alpine3.11 AS build-env
RUN apk --no-cache add ca-certificates
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum main.go /app/
RUN go build -o vs2kubes

FROM alpine
WORKDIR /app
COPY --from=build-env /app/vs2kubes /app/
ENTRYPOINT /app/vs2kubes
