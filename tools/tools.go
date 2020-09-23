// +build tools

package main

import (
	// for vendoring CI linting tools
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
)

func main() {
}
