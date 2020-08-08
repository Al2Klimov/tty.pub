FROM golang:alpine as server

COPY server /server
WORKDIR /server
RUN ["go", "build", "."]


FROM alpine
RUN ["apk", "add", "imagemagick", "ttf-liberation"]

COPY --from=server /server/server /server
CMD ["/server"]
