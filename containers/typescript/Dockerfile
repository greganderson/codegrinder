FROM alpine:edge

RUN mkdir /home/student && chmod 777 /home/student
WORKDIR /home/student

RUN apk add --no-cache \
    icdiff \
    make \
    nodejs \
    npm

RUN npm install -g typescript ts-node

# Create a package.json with the required dependencies
COPY files/typescriptunittest/package.json /home/student/
COPY files/typescriptunittest/tsconfig.json /home/student/
RUN npm install

# Clean up npm cache to reduce image size
RUN npm cache clean --force
