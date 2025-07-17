#!/usr/bin/make -f

build:
	go build -o ./build/xray
.PHONY: build

run:
	./build/xray
.PHONY: run