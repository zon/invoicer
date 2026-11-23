# Stack

Invoicer has a CLI and a web app. Both use the same SQLite database. The CLI generates invoices. The web app lists and renders them.

The web app renders HTML on the server with Go's `html/template` package. Gin serves HTTP. There is no client-side framework or JavaScript build step. The binary embeds the stylesheet.

The web app also renders invoice PDFs in-process with gopdf. Rendering needs no browser and no external tool.

## Software

* [Go](https://go.dev/)
* [Gin](https://gin-gonic.com/)
* [gopdf](https://github.com/signintech/gopdf)
* [Resend](https://resend.com/) (invoice email)
* [Resend Go SDK](https://github.com/resend/resend-go) (invoice email)
* [Kong](https://github.com/alecthomas/kong)
* [yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3)
* [GORM](https://gorm.io/)
* [SQLite](https://sqlite.org/)
* [just](https://just.systems/man/en/)
* [air](https://github.com/air-verse/air) (development only)

## Kubernetes Addons

* [cert-manager](https://cert-manager.io/)
* [Argo Workflows](https://argoproj.github.io/workflows/) (monthly schedule)
