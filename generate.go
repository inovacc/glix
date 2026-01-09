package main

//go:generate go get github.com/inovacc/genversioninfo
//go:generate go run ./scripts/genversion/genversion.go

//go:generate go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
//go:generate sqlc generate
