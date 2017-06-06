FROM python:3.5

RUN apt-get update && \
    apt-get install -y supervisor && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir -p /usr/src/app/magneticod && \
    mkdir -p /usr/src/app/magneticow

WORKDIR /usr/src/app

ADD magneticod/requirements.txt magneticod/requirements.txt
ADD magneticow/requirements.txt magneticow/requirements.txt

RUN cd magneticod && \
    pip install -r requirements.txt && \
    cd ../magneticow && \
    pip install -r requirements.txt && \
    cd ..

COPY . .

EXPOSE 8080

ENTRYPOINT ["/usr/bin/supervisord", "-c", "/usr/src/app/.docker/supervisord.conf"]

