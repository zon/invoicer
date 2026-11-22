# Config Design

Config holds user defaults for the CLI. It lives at `~/.invoicer/config.yaml` and uses YAML. The file is optional.

## Values

Config holds these values:

* `vendor` — name of the contractor sending invoices
* `customer` — name of the client receiving invoices
* `project` — optional project name
* `rate` — hourly rate in dollars
* `hours` — hours worked per week
* `database` — path to the invoice database

`database` defaults to `~/.invoicer/invoices.db`. The other values have no default. Generate requires `vendor`, `customer`, `rate`, and `hours` from a flag or config. `project` may be empty.

The `pdf` and `model` values are removed. See [Invoices](invoices.md).

## Precedence

Generate merges values in this order, highest first:

1. command-line flags
2. config values
3. built-in defaults

A flag wins over the matching config value. A missing config file is an empty config. Invalid YAML is an error.

A missing or zero `rate` or `hours` is unset. Unknown config keys are ignored.

## File

```yaml
vendor: Jane Smith
customer: Acme Corp
project: Website Redesign
rate: 150
hours: 40
database: /var/lib/invoicer/invoices.db
```

The config path is fixed. Generate does not write the file.

## Set Config

`invoicer set config` writes values to the config file. It accepts the same options as generate, without the month and year arguments.

Only provided flags update the file. Other values stay as they are. The command creates `~/.invoicer` and the file when they do not exist.

`set config` cannot clear a value. Edit the file to remove one.
