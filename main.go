package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
)

const agoUsage = `usage: ago <command> [arguments]

ago is a wrapper around the go command that adds the ability to alias packages
with short, memorable names. Only the get and install commands are affected. All
other flags and arguments are passed through to the go command.

create aliases with the alias command:

	$ ago alias foo github.com/foo/bar/v2

then use the alias in place of the package name:

	$ ago get foo

which is equivalent to:

	$ go get github.com/foo/bar/v2

The commands are:

	alias, a      create/manage package aliases
	get           download packages and dependencies
	install       compile and install packages and dependencies
	help          display this help text

`

const aliasUsage = `usage:

create an alias:

	ago alias foo github.com/foo/bar/v2

remove an alias:

	ago alias rm foo

list all aliases:

	ago alias list

The sub-commands are:

	list, ls, l       list all aliases
	rm                remove an alias
	help	          display this help text

`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(agoUsage)
		return
	}

	aliases, err := loadAliases()
	if err != nil {
		fatalf("error: %v", err)
	}

	args := make([]string, len(os.Args))
	copy(args, os.Args)

	switch args[1] {
	case "help":
		fmt.Print(agoUsage)
		return
	case "get", "install":
		if len(args) > 2 {
			for i := 2; i < len(args); i++ {
				arg := args[i]

				// Find the alias with the longest matching prefix.
				var alias string
				var pkg string
				for a, p := range aliases {
					if strings.HasPrefix(arg, a) && len(a) > len(alias) {
						alias = a
						pkg = p
					}
				}
				if alias == "" {
					continue
				}

				// If the user is requesting a specific version, extract it.
				var version string
				if idx := strings.LastIndex(arg, "@"); idx != -1 {
					version = arg[idx:]
					arg = arg[:idx]
				}

				pkgPath := strings.TrimPrefix(arg, alias)

				// If the package path starts with a major version, then we need
				// to strip it off and replace it with the aliased package path.
				var major string
				if split := strings.SplitN(pkgPath, "/", 3); len(split) > 1 {
					if split[1][0] == 'v' {
						if _, err := strconv.Atoi(split[1][1:]); err == nil {
							major = "/" + split[1]
							if len(split) > 2 {
								pkgPath = "/" + split[2]
							} else {
								pkgPath = ""
							}
						}
					}
				}

				// If the user has requested a specific major version, and the
				// aliased package path already contains a major version, then
				// we need to strip it off and replace it with the requested
				// major version. Unless the requested major version < 2, in
				// which case we just strip it off.
				if major != "" {
					// Strip off the major version.
					if idx := strings.LastIndex(pkg, "/v"); idx != -1 {
						if _, err := strconv.Atoi(pkg[idx+2:]); err == nil {
							pkg = pkg[:idx]
						}
					}

					// If the requested major version is < 2, then set it to
					// the empty string.
					if len(major) == 3 && (major[2] == '0' || major[2] == '1') {
						major = ""
					}
				}

				args[i] = pkg + major + pkgPath + version
			}
		}
	case "alias", "a":
		if len(args) < 3 {
			fmt.Print(aliasUsage)
			return
		}
		switch args[2] {
		case "help":
			fmt.Print(aliasUsage)
			return
		case "list", "ls", "l":
			type row struct {
				alias string
				pkg   string
			}
			var rows []row
			for alias, pkg := range aliases {
				rows = append(rows, row{alias, pkg})
			}
			sort.Slice(rows, func(i, j int) bool {
				return rows[i].alias < rows[j].alias
			})

			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ALIAS\tPACKAGE")
			fmt.Fprintln(tw, "-----\t-------")
			for _, row := range rows {
				fmt.Fprintf(tw, "%s\t%s\n", row.alias, row.pkg)
			}
			tw.Flush()
			return
		case "rm":
			if len(args) < 4 {
				fatalf("error: not enough arguments")
			}
			delete(aliases, args[3])
			if err := storeAliases(aliases); err != nil {
				fatalf("error: %v", err)
			}
			fmt.Printf("removed alias %q\n", args[3])
			return
		default:
			if len(args) < 4 {
				fatalf("error: not enough arguments")
			}
			aliases[args[2]] = args[3]
			if err := storeAliases(aliases); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("aliased %q to %q\n", args[2], args[3])
			return
		}
	}

	fmt.Printf("> go %s\n", strings.Join(args[1:], " "))

	cmd := exec.Command("go", args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if ok := errors.As(err, &exitErr); ok {
			os.Exit(exitErr.ExitCode())
		}
		fatalf("error: %v", err)
	}
}

const aliasesFile = "aliases.json"

func loadAliases() (map[string]string, error) {
	f, err := os.Open(filepath.Join(configDir, aliasesFile))
	if os.IsNotExist(err) {
		return make(map[string]string), nil
	}
	if err != nil {
		return nil, fmt.Errorf("open aliases file: %w", err)
	}
	defer f.Close()

	var aliases map[string]string
	if err := json.NewDecoder(f).Decode(&aliases); err != nil {
		return nil, fmt.Errorf("decode aliases file: %w", err)
	}
	return aliases, nil
}

func storeAliases(aliases map[string]string) error {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	f, err := os.Create(filepath.Join(configDir, aliasesFile))
	if err != nil {
		return fmt.Errorf("create aliases file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(aliases); err != nil {
		return fmt.Errorf("encode aliases file: %w", err)
	}
	return nil
}

func fatalf(format string, args ...interface{}) {
	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

var configDir string

func init() {
	if configDir = os.Getenv("AGO_CONFIG_DIR"); configDir != "" {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	configDir = filepath.Join(home, ".ago")
}
