FROM golang:1.26-alpine AS build

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /app/sli-backend ./cmd/server

FROM alpine:3.22

# chromium: headless-rendered marksheet PDFs (internal/platform/pdf/generator.go,
# driven via chromedp). ttf-freefont: without any fonts installed, Chromium
# renders text as empty boxes.
RUN apk add --no-cache ca-certificates chromium ttf-freefont && \
    addgroup -S app && adduser -S app -G app
WORKDIR /app
COPY --from=build /app/sli-backend /app/sli-backend
RUN mkdir -p /app/data/marksheets && chown -R app:app /app
USER app
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/app/sli-backend"]
