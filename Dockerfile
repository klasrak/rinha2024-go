FROM golang:1.22 AS builder

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /build

COPY go.mod .
COPY go.sum .

RUN go mod download && go mod tidy

COPY . .

# Build the application
# RUN go build -o tax cmd/tax/main.go
RUN go build -tags=jsoniter -o . main.go

WORKDIR /dist

RUN cp /build/main .

# Build a small image
FROM ubuntu:latest as final

# Copy binary from build to main folder
COPY --from=builder /dist/main /

# Command to run
CMD [ "./main" ]
