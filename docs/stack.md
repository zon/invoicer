# Stack

Invoicer has a CLI and a web app. Both use the same SQLite database. The CLI generates invoices. The web app lists and renders them.

The web app renders HTML on the server with Go's `html/template` package. Gin serves HTTP. htmx swaps in HTML fragments for partial page updates. There is no client-side framework or JavaScript build step. The binary embeds htmx and the stylesheet.

## Software

* [Go](https://go.dev/)
* [Gin](https://gin-gonic.com/)
* [htmx](https://htmx.org/)
* [Kong](https://github.com/alecthomas/kong)
* [yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3)
* [GORM](https://gorm.io/)
* [SQLite](https://sqlite.org/)
* [just](https://just.systems/man/en/)
