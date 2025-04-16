FROM alpine:edge

RUN mkdir /home/student && chmod 777 /home/student
WORKDIR /home/student

RUN apk add --no-cache \
    icdiff \
    make \
    nodejs \
    npm

RUN npm install -g typescript ts-node

# Clean up npm cache to reduce image size
RUN npm cache clean --force
