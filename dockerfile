FROM golang:latest AS build

WORKDIR /usr/src/app

ENV GOOS=linux
ENV GOARCH=amd64 

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o /usr/src/app/run.bin ./cmd

FROM gcr.io/distroless/static-debian11
COPY --from=build /usr/src/app/run.bin /usr/local/bin/
CMD ["run.bin"]