# ago

ago is a wrapper around the go command that adds the ability to alias packages
with short, memorable names. Only the get and install commands are affected. All
other flags and arguments are passed through to the go command.

## Installation

    go install github.com/deitrix/ago@latest

## Configuration

By default, ago stores aliases in `$HOME/.ago/`. This can be changed by setting
the `AGO_CONFIG_DIR` environment variable.

## Usage

Define a package alias:
    
    ago alias github.com/username/project myproject

Get a package using the alias:

    ago get myproject

List package aliases:

    ago alias ls

Remove a package alias:

    ago alias rm myproject
