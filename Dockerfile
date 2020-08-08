FROM golang:alpine as server

COPY server /server
WORKDIR /server
RUN ["go", "build", "."]


FROM node as xtermjs
RUN ["npm", "install", "-g", "clean-css-cli"]

RUN ["mkdir", "/xterm.js"]
WORKDIR /xterm.js
RUN ["npm", "install", "xterm"]

RUN ["mkdir", "css"]
RUN ["cleancss", "--source-map", "-o", "css/xterm.css", "node_modules/xterm/css/xterm.css"]


FROM node as client
RUN ["npm", "install", "-g", "clean-css-cli", "uglify-js"]

COPY client /client
WORKDIR /client
RUN ["mkdir", "min"]

RUN ["uglifyjs", "main.js", "-c", "-m", "-o", "min/main.js", "--source-map"]
RUN ["cleancss", "--source-map", "-o", "min/style.css", "style.css"]


FROM alpine
RUN ["apk", "add", "imagemagick", "ttf-liberation"]

COPY --from=client /client/min /www
COPY --from=xtermjs /xterm.js/node_modules/xterm/lib/xterm.js* /xterm.js/css/xterm.css* /www/

COPY --from=server /server/server /server
CMD ["/server"]
