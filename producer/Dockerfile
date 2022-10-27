FROM quay.io/app-sre/golang:1.18.5 as builder
WORKDIR /build
COPY . .
RUN make build

FROM registry.access.redhat.com/ubi8-minimal
COPY --from=builder /build/gitlab-sync-s3-push  /bin/gitlab-sync-s3-push
RUN microdnf update -y && microdnf install -y git && microdnf install -y ca-certificates

ENTRYPOINT  [ "/bin/gitlab-sync-s3-push" ]