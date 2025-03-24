// Command-line tool to run the various authz examples
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/dpup/prefab/examples/authz/common-builder"
	"github.com/dpup/prefab/examples/authz/custom"
)

func main() {
	// Parse command-line arguments
	exampleType := flag.String("example", "common-builder", "Type of example to run: custom, common-builder")
	flag.Parse()

	// Run the selected example
	switch *exampleType {
	case "custom":
		// Run the fully custom configuration example
		fmt.Println("Running custom authorization example")
		fmt.Println("Run curl commands as shown in the example file")
		custom.Run()
	case "common-builder":
		// Run the common builder pattern example
		fmt.Println("Running common builder authorization example")
		fmt.Println("Run curl commands as shown in the example file")
		commonbuilder.Run()
	default:
		fmt.Printf("Unknown example type: %s\n", *exampleType)
		fmt.Println("Available examples: custom, common-builder")
		os.Exit(1)
	}
}