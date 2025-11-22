FROM alpine:latest

ARG TARGETARCH

ENV TZ=Asia/Shanghai

RUN apk --no-cache add ca-certificates \
	tzdata

WORKDIR /app

ADD ./build/linux_${TARGETARCH}/bin ./
ADD ./www /app/www

LABEL Name=gowvp Version=0.0.1

EXPOSE 15123

CMD [ "./bin" ]