all: build deploy

build:
	GOOS=linux GOARCH=arm GOARM=6 go build -o arm/temp-sensor-scanner

deploy:
	scp arm/temp-sensor-scanner zero.local:/opt/scanner/
