// Command fixturetool implements the fixture's custom build actions, so no
// action depends on host tools.
//
//	fixturetool bundle --out FILE --header TEXT [--blank] SRC...
//	fixturetool upper IN OUT
//	fixturetool stdout --out FILE -- TOOL ARG...
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
)

func main() {
	if len(os.Args) < 2 {
		fail(fmt.Errorf("usage: fixturetool bundle|upper|stdout ..."))
	}
	var err error
	switch os.Args[1] {
	case "bundle":
		err = bundle(os.Args[2:])
	case "upper":
		err = upper(os.Args[2:])
	case "stdout":
		err = stdout(os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "fixturetool:", err)
	os.Exit(1)
}

func bundle(args []string) error {
	fs := flag.NewFlagSet("bundle", flag.ContinueOnError)
	out := fs.String("out", "", "output file")
	header := fs.String("header", "# bundle", "first line")
	blank := fs.Bool("blank", false, "add a blank line after the header")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var b bytes.Buffer
	b.WriteString(*header + "\n")
	if *blank {
		b.WriteString("\n")
	}
	for _, src := range fs.Args() {
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		b.Write(data)
	}
	return os.WriteFile(*out, b.Bytes(), 0o644)
}

func upper(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: upper IN OUT")
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	return os.WriteFile(args[1], bytes.ToUpper(data), 0o644)
}

func stdout(args []string) error {
	fs := flag.NewFlagSet("stdout", flag.ContinueOnError)
	out := fs.String("out", "", "output file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("usage: stdout --out FILE -- TOOL ARG...")
	}
	cmd := exec.Command(fs.Arg(0), fs.Args()[1:]...)
	cmd.Stderr = os.Stderr
	data, err := cmd.Output()
	if err != nil {
		return err
	}
	return os.WriteFile(*out, data, 0o644)
}
