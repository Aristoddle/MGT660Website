docker.exe stop test
docker.exe build -t outyet .
docker.exe run --publish 6060:8080 --name test --rm outyet
