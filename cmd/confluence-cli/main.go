// Command confluence-cli lets Coding Agents use a Confluence instance as an
// external knowledge base: read pages, search via CQL, and manage comments.
package main

import (
	"os"

	"github.com/angelmsger/confluence-cli/internal/app"
)

func main() {
	os.Exit(app.Execute())
}
