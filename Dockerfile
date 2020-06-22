FROM golang:1.14.2-alpine3.11 AS build-env
RUN apk --no-cache add ca-certificates
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum main.go /app/
RUN go build -o vs2kubes

FROM alpine
RUN mkdir -p /home/app
RUN addgroup app && \
    adduser --uid 1001 --ingroup app --home /home/app --shell /bin/sh app
ENV HOME=/home/app
WORKDIR $HOME
COPY --from=build-env /app/vs2kubes $HOME
RUN chown -R app:app $HOME
USER app
ENTRYPOINT /$HOME/vs2kubes
