package generate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"
	"github.com/ydnar/wasm-tools-go/internal/codec"
	"github.com/ydnar/wasm-tools-go/internal/go/gen"
	"github.com/ydnar/wasm-tools-go/internal/witcli"
	"github.com/ydnar/wasm-tools-go/wit/bindgen"
)

// Command is the CLI command for wit.
var Command = &cli.Command{
	Name:    "generate",
	Aliases: []string{"go"},
	Usage:   "generates Go from a fully-resolved WIT JSON file",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:      "out",
			Aliases:   []string{"o"},
			Value:     ".",
			TakesFile: true,
			OnlyOnce:  true,
			Usage:     "output directory",
		},
		&cli.StringMapFlag{
			Name:  "map",
			Usage: "maps WIT identifiers to Go identifiers",
		},
		&cli.BoolFlag{
			Name:  "versioned",
			Usage: "emit versioned Go package(s) for each WIT version",
		},
		&cli.BoolFlag{
			Name:  "dry-run",
			Usage: "do not write files; print to stdout",
		},
	},
	Action: action,
}

func action(ctx context.Context, cmd *cli.Command) error {
	dryRun := cmd.Bool("dry-run")

	out := cmd.String("out")
	info, err := os.Stat(out)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", out)
	}
	fmt.Fprintf(os.Stderr, "Output dir: %s\n", out)
	outPerm := info.Mode().Perm()

	pkgPath, err := gen.PackagePath(out)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Package root: %s\n", pkgPath)

	res, err := witcli.LoadOne(cmd.Args().Slice()...)
	if err != nil {
		return err
	}

	packages, err := bindgen.Go(res,
		bindgen.GeneratedBy(cmd.Root().Name),
		bindgen.PackageRoot(pkgPath),
		bindgen.Versioned(cmd.Bool("versioned")),
		bindgen.MapIdents(cmd.StringMap("map")),
	)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Generated %d package(s)\n", len(packages))

	for _, pkg := range packages {
		if !pkg.HasContent() {
			fmt.Fprintf(os.Stderr, "Skipping empty package: %s\n", pkg.Path)
			continue
		}

		fmt.Fprintf(os.Stderr, "Generated package: %s\n", pkg.Path)

		for _, filename := range codec.SortedKeys(pkg.Files) {
			file := pkg.Files[filename]

			dir := filepath.Join(out, strings.TrimPrefix(file.Package.Path, pkgPath))
			err := os.MkdirAll(dir, outPerm)
			if err != nil {
				return err
			}

			b, err := file.Bytes()
			if err != nil {
				return err
			}

			path := filepath.Join(dir, file.Name)
			fmt.Fprintf(os.Stderr, "Generated file: %s\n", path)

			if dryRun {
				fmt.Println(string(b))
				fmt.Println()
				continue
			}

			f, err := os.Create(path)
			if err != nil {
				return err
			}
			n, err := f.Write(b)
			f.Close()
			if err != nil {
				return err
			}
			if n != len(b) {
				return fmt.Errorf("wrote %d bytes to %s, expected %d", n, path, len(b))
			}
		}
	}

	return nil
}
