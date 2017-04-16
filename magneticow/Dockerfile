FROM python:3.6

RUN mkdir -p /usr/src/app

WORKDIR /usr/src/app

COPY . .
RUN pip install -e .
EXPOSE 8080
CMD ["python", "-mmagneticow", "--port", "8080", "--user", "user", "password"]
