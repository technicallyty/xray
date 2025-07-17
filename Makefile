#!/usr/bin/make -f

build:
	go build -o build
.PHONY: build

run:
	./build/xray
.PHONY: run