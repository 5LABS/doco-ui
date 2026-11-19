FROM node:22-alpine AS frontend
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
COPY assets/ /src/assets/
RUN npm run build

FROM golang:1.26-alpine AS backend
WORKDIR /src
COPY go.mod *.go ./
COPY --from=frontend /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -o /doco-ui .

FROM scratch
COPY --from=backend /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=backend /doco-ui /doco-ui
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/doco-ui"]
