# Golang Installer [![Test](https://github.com/inovacc/goinstall/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/inovacc/goinstall/actions/workflows/test.yml)

this app is a `goinstall` app that is a wrapper around `go install` plus `sqlite` database to handle all modules
installed and eventually monitoring then for updates

## To install

````shell
go install github.com/inovacc/goinstall@latest
````

## command to run install

```shell
goinstall https://github.com/inovacc/ksuid/cmd/ksuid.git
goinstall https://github.com/inovacc/ksuid/cmd/ksuid
goinstall git://github.com/inovacc/ksuid/cmd/ksuid
goinstall ssh://github.com/inovacc/ksuid/cmd/ksuid
goinstall github.com/inovacc/ksuid/cmd/ksuid@latest

Fetching module information...
Installing module: github.com/inovacc/ksuid/cmd/ksuid
Module is installer successfully: github.com/inovacc/ksuid/cmd/ksuid
Show report using goinstall report github.com/inovacc/ksuid/cmd/ksuid
```

## Roadmap

[x] install module

[ ] report

[ ] monitoring

[ ] auto update
