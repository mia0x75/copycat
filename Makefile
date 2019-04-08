BINARY=nova
export GO111MODULE=on
GOPATH ?= $(shell go env GOPATH)
ifeq "$(GOPATH)" ""
  $(error 运行`make`命令前请先设置好GOPATH环境变量。)
endif
PATH := ${GOPATH}/bin:$(PATH)

GO_VERSION_MIN=1.10

.PHONY: all
all: | fmt build

# TODO: 依赖检查
.PHONY: deps
deps:
	@echo "\033[92mDependency check\033[0m"
	@bash ./deps.sh
	# The retool tools.json is setup from retool-install.sh
	retool sync
	retool do gometalinter.v2 intall

# 代码格式化
.PHONY: fmt
fmt:
	@echo "\033[92mRun gofmt on all source files ...\033[0m"
	@echo "gofmt -l -s -w ..."
	@ret=0 && for d in $$(go list -f '{{.Dir}}' ./... | grep -v /vendor/); do \
		gofmt -l -s -w $$d/*.go || ret=$$? ; \
	done ; exit $$ret

# 运行全部测试用例
.PHONY: test
test:
	@echo "\033[92mRun all test cases ...\033[0m"
	go test ./...
	@echo "test Success!"

# 测试代码覆盖率
.PHONY: cover
cover: test
	@echo "\033[92mRun test cover check ...\033[0m"
	go test -coverpkg=./... -coverprofile=coverage.data ./... | column -t
	go tool cover -html=coverage.data -o coverage.html
	go tool cover -func=coverage.data -o coverage.txt
	@tail -n 1 coverage.txt | awk '{sub(/%/, "", $$NF); \
		if($$NF < 80) \
			{print "\033[91m"$$0"%\033[0m"} \
		else if ($$NF >= 90) \
			{print "\033[92m"$$0"%\033[0m"} \
		else \
			{print "\033[93m"$$0"%\033[0m"}}'

# 项目构建
build: fmt
	@echo "\033[92mBuilding ...\033[0m"
	@bash ./genver.sh $(GO_VERSION_MIN)
	@ret=0 && for d in $$(go list -f '{{if (eq .Name "main")}}{{.ImportPath}}{{end}}' ./); do \
		CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -mod=readonly -ldflags="-s -w" -race -o bin/$(BINARY) $$d || ret=$$? ; \
	done ; exit $$ret
	@echo "build Success!"

# 安装
install: build
	@echo "\033[92mInstall ...\033[0m"
	go install ./...
	@echo "install Success!"

# gometalinter
# 如果有不想改的lint问题可以使用metalinter.sh加黑名单
#@bash doc/example/metalinter.sh
.PHONY: lint
lint: build
	@echo "\033[92mRun linter check ...\033[0m"
	CGO_ENABLED=0 retool do gometalinter.v2 -j 1 --config doc/example/metalinter.json ./...
	retool do revive -formatter friendly --exclude vendor/... -config doc/example/revive.toml ./...
	retool do golangci-lint --tests=false run
	@echo "gometalinter check your code is pretty good"

# 清理
.PHONY: clean
clean:
	@echo "\033[92mCleanup ...\033[0m"
	go clean
	rm -f ${BINARY}
	rm -f ${BINARY} coverage.*
	find . -name "*.log" -delete
	git clean -fi
