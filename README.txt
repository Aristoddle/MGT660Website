docker.exe build -t outyet .
docker.exe run --publish 6060:8080 --name test_no_x --rm outyet
