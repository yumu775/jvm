package cmd

// Version 可以通过 go build -ldflags "-X jvm/cmd.Version=..." 注入发布版本。
var Version = "1.1.0-dev"
