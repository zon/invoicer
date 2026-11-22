# Config Specification

## Purpose
The YAML config file at `~/.invoicer/config.yaml` stores user defaults for invoice generation.

## Requirements

### Requirement: Configuration Values
The config SHALL hold `vendor`, `customer`, `project`, `rate`, `hours`, and `database`.

`database` SHALL default to `~/.invoicer/invoices.db`. The other values SHALL have no default. Generate SHALL require `vendor`, `customer`, `rate`, and `hours` from a flag or config. `project` MAY be empty.

The system SHALL ignore `pdf` and `model` keys in a config file.

#### Scenario: Config supplies required values
- GIVEN a config with vendor, customer, rate, and hours
- WHEN generate runs without those flags
- THEN generate uses the config values

#### Scenario: Missing required value
- GIVEN a config without a vendor
- WHEN generate runs without `--vendor`
- THEN an error reports that vendor is required

#### Scenario: Removed values are ignored
- GIVEN a config with `pdf: true` and `model: deepseek/deepseek-flash`
- WHEN generate runs
- THEN neither value affects the invoice

### Requirement: Value Order
Generate SHALL resolve values in this order, highest first: command-line flags, config values, built-in defaults. A flag SHALL win over the matching config value.

#### Scenario: Flag overrides config
- GIVEN a config rate of 150
- WHEN generate runs with `--rate 200`
- THEN the invoice uses 200 as the hourly rate

#### Scenario: Config fills an omitted flag
- GIVEN a config database path
- WHEN generate runs without `--database`
- THEN generate uses the config path

### Requirement: Config File
Loading a missing config file SHALL produce an empty config without error. Invalid YAML SHALL produce an error. The system SHALL ignore unknown config keys. The system SHALL treat a missing or zero `rate` or `hours` as unset.

#### Scenario: First run has no config
- GIVEN no config file exists
- WHEN generate runs with all required values as flags
- THEN generate creates the invoice without a config error

#### Scenario: Invalid YAML
- GIVEN a config file with invalid YAML
- WHEN generate runs
- THEN the command reports a config error
- AND the command stores no invoice

### Requirement: Database Location
The CLI SHALL choose the database path from `--database`, then the `database` config value, then `~/.invoicer/invoices.db`.

The CLI SHALL create the database file and its parent directory when they do not exist.

#### Scenario: Database flag wins
- GIVEN a config database path
- WHEN generate runs with a different `--database` path
- THEN the CLI stores the invoice at the flag path

#### Scenario: Default path
- GIVEN no database is configured or passed
- WHEN an invoice is saved
- THEN the CLI creates the database at `~/.invoicer/invoices.db`

#### Scenario: Missing directory is created
- GIVEN a database path in a directory that does not exist
- WHEN an invoice is saved
- THEN the CLI creates the directory and database file

### Requirement: Write Configuration
`invoicer set config` SHALL write provided values to the config file. It SHALL accept `vendor`, `customer`, `project`, `rate`, `hours`, and `database`. It SHALL update only the values provided, leaving others unchanged, and SHALL create `~/.invoicer` and the config file when missing.

`set config` SHALL NOT clear a value. Removing a value SHALL require editing the file.

#### Scenario: Update one value
- GIVEN a config with a vendor and rate
- WHEN `set config --rate 200` runs
- THEN the rate is 200
- AND the vendor stays the same

#### Scenario: Writes database path
- GIVEN no config file
- WHEN `set config --database /var/lib/invoicer/invoices.db` runs
- THEN the command creates the config file with that database path
- AND the `~/.invoicer` directory exists

#### Scenario: Unspecified values unchanged
- GIVEN a config with every value set
- WHEN `set config --hours 32` runs
- THEN only hours changes
