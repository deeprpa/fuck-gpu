define echo_green
    printf "\e[38;5;40m"
    echo "${1}"
    printf "\e[0m \n"
endef

#项目名
Project=fuck-gpu

BINARY=${Project}

Path=github.com/deeprpa/${Project}/version
#当前版本号,每次更新服务时都必须更新版本号， 或使用 tag, 更新 tag
# Version=v1.0.1
Version=$(shell git describe --tags)
GitCommit=$(shell git rev-parse --short HEAD || echo unsupported)
GoVersion=$(shell go version)
BuildTime=$(shell date "+%Y-%m-%d_%H:%M:%S")

current_dir=$(shell pwd)
#pb_go_files=./pb/common.pb.go ./pb/service.pb.go
#goflags=GOFLAGS="-mod=readonly"

# Setup the -ldflags option for go build here, interpolate the variable values
LDFLAGS=-ldflags "-w -s \
	-X ${Path}.Version=${Version} \
	-X '${Path}.GitCommit=${GitCommit}' \
	-X '${Path}.GoVersion=${GoVersion}' \
	-X '${Path}.BuildTime=${BuildTime}'"

clean:
	rm -rf ./build

build: clean
	-go build -o ./build/panic ./tests/panic/
	${goflags} go build ${LDFLAGS} -v -o ./build/${Project}
	# @ before command only output result
	@echo "build finish !!!"
	@echo "Version:   " $(Version)
	@echo "Git commit:" $(GitCommit)
	@echo "Go version:" $(GoVersion)
	@echo "Build time:" $(BuildTime)
	@${call echo_green,"build finished! The target is ${current_dir}/build/${Project}."}

run:
	${goflags} go run ${LDFLAGS} main.go -D d 

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o build/${Project} -v
	# @ before command only output result
	@echo "build finish !!!"
	@echo "Version:   " $(Version)
	@echo "Git commit:" $(GitCommit)
	@echo "Go version:" $(GoVersion)
	@echo "Build time:" $(BuildTime)
	@${call echo_green,"build finished! The target is ${current_dir}/build/${Project}."}

images:
	docker build -t registry.cn-hangzhou.aliyuncs.com/pedge-platform/${Project}:${Version} .

push: images
	docker push registry.cn-hangzhou.aliyuncs.com/pedge-platform/${Project}:${Version}

release-images: push
	docker tag registry.cn-hangzhou.aliyuncs.com/pedge-platform/${Project}:${Version} registry.cn-hangzhou.aliyuncs.com/pedge-platform/${Project}:latest
	docker push registry.cn-hangzhou.aliyuncs.com/pedge-platform/${Project}:latest

ver:
	@echo "Version:   " $(Version)
	@echo "Git commit:" $(GitCommit)
	@echo "Go version:" $(GoVersion)
	@echo "Build time:" $(BuildTime)

