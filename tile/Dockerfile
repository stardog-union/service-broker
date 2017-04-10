FROM cfplatformeng/tile-generator

RUN apk add --no-cache curl

RUN curl -L "https://cli.run.pivotal.io/stable?release=linux64-binary&source=github" | tar -zx \
  && mv cf /usr/bin/cf
