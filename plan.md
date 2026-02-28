a go cli app for generating invoices for an hourly contractor. uses kong for cli parsing. prompts opencode to generate html invoices with random styling.

args:
- month - text or numeric month to invoice for. defaults to previous month
- year - year of the month to invoice for. defaults to closest year to the month

options:
- vendor - name of the contractor sending the invoice. required without config
- customer - name of the client receiving the invoice. required without config
- rate - hourly rate. required without config
- hours - hours per week worked. required without config
- pdf - if the html should be converted to pdf. (we will need to pick a conversion tool.) defaults to false
- model - opencode formatted model stub to prompt. defaults to anthropic/claude-haiku-4-5

invoices cover one month. with weekly items. a week belongs to a month if it's wednesday is in that month. options can be specified in an ~/.invoicer/config.yaml file. options passed to the app take precedence over those in the config file.

docs:
- README.md documents all args, options, subcommands, and the config file
- the app itself provides help text for all args, options, and subcommands via kong

build:
- makefile with `make install` that builds and installs the binary to $GOPATH/bin

subcommands:
- set config - takes the same options as the main command (vendor, customer, rate, hours, pdf, model) and writes them to ~/.invoicer/config.yaml, creating it if it doesn't exist. only specified options are written; unspecified options are left unchanged.