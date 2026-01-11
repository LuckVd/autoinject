package main

import (
	"iast-auto-inject/cmd"
	"iast-auto-inject/internal/pkg/logger"
)

func main() {
	defer logger.Sync()
	cmd.Execute()
}
