//go:build tools

//go:generate go get github.com/inovacc/genversioninfo
//go:generate go run ./scripts/genversion/genversion.go

//go:generate go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
//go:generate go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
//go:generate go run ./scripts/proto/generate.go

package tools

import (
	_ "github.com/inovacc/genversioninfo"
)
