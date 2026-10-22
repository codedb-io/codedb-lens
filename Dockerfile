# ghcr.io/codedb-io/codedb-lens — built by the release workflow from the released Linux archives (bin/<arch>/).
FROM gcr.io/distroless/static-debian12:nonroot
ARG TARGETARCH
COPY bin/${TARGETARCH}/codedb-lens /usr/bin/codedb-lens
# Images are updated by pulling a new tag, never in place.
ENV CODEDB_LENS_NO_UPDATE=1
ENTRYPOINT ["/usr/bin/codedb-lens"]
