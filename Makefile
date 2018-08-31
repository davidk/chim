all: fmt dep test coverage clean_bin build clean_arm arm

.PHONY: vet
vet: 
	go vet

.PHONY: dep 
dep: 
	go get -v github.com/golang/dep/cmd/dep
	$(GOPATH)/bin/dep ensure

.PHONY: depupdate
depupdate:
	$(GOPATH)/bin/dep ensure -update

depstatus:
	$(GOPATH)/bin/dep status	

.PHONY: arm
arm: clean_arm build_arm

.PHONY: clean_bin
clean_bin:
	rm -f bin/chim

.PHONY: clean_arm
clean_arm:
	rm -f bin/linux_arm/chim

.PHONY: build_arm
build_arm:
	GOARCH=arm go build -o bin/linux_arm/chim -ldflags "-X main.gitCommit=$$(git describe --abbrev=10 --dirty --always --tags)-$$(git rev-parse --abbrev-ref HEAD)" github.com/davidk/chim

.PHONY: build
build:
	go build -o bin/chim -ldflags "-X main.gitCommit=$$(git describe --abbrev=10 --dirty --always --tags)-$$(git rev-parse --abbrev-ref HEAD)" github.com/davidk/chim

.PHONY: test
test:
	go test -tags test -coverprofile=coverage.out github.com/davidk/chim

.PHONY: verbose_test
verbose_test:
	go test -v -tags test -coverprofile=coverage.out github.com/davidk/chim

.PHONY: coverage
coverage:
	go tool cover -html=coverage.out -o coverage.html

.PHONY: fmt
fmt:
	go fmt github.com/davidk/chim
