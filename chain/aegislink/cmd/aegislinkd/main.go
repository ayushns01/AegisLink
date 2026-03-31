package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ayushns01/aegislink/chain/aegislink/app"
)

func main() {
	a := app.New()
	_, _ = fmt.Fprintf(
		os.Stdout,
		"%s initialized with modules: %s\n",
		a.Config.AppName,
		strings.Join(a.ModuleNames(), ", "),
	)
}
