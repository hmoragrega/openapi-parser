package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/gobwas/cli"
	openapiparser "github.com/hmoragrega/openapi-parser"
	"gopkg.in/yaml.v3"
)

func main() {
	cli.Main(cli.Commands{
		"generate": new(generateCmd),
	})
}

type generateCmd struct {
	specPath string
	outPath  string
}

func (c *generateCmd) DefineFlags(fs *flag.FlagSet) {
	fs.StringVar(
		&c.specPath,
		"in",
		c.specPath,
		"input spec filepath",
	)
}

func (c *generateCmd) Run(_ context.Context, _ []string) error {
	s, err := openapiparser.Parse(c.specPath)
	if err != nil {
		return err
	}

	y, err := yaml.Marshal(s)
	if err != nil {
		return err
	}

	_, err = fmt.Println(string(y))
	return err
}
