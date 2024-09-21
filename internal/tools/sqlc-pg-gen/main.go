package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/jackc/pgx/v4"
)

// https://dba.stackexchange.com/questions/255412/how-to-select-functions-that-belong-in-a-given-extension-in-postgresql
//
// Extension functions are added to the public schema
const extensionFuncs = `
WITH extension_funcs AS (
  SELECT p.oid
  FROM pg_catalog.pg_extension AS e
      INNER JOIN pg_catalog.pg_depend AS d ON (d.refobjid = e.oid)
      INNER JOIN pg_catalog.pg_proc AS p ON (p.oid = d.objid)
      INNER JOIN pg_catalog.pg_namespace AS ne ON (ne.oid = e.extnamespace)
      INNER JOIN pg_catalog.pg_namespace AS np ON (np.oid = p.pronamespace)
  WHERE d.deptype = 'e' AND e.extname = $1
)
SELECT p.proname as name,
  format_type(p.prorettype, NULL),
  array(select format_type(unnest(p.proargtypes), NULL)),
  p.proargnames,
  p.proargnames[p.pronargs-p.pronargdefaults+1:p.pronargs],
  p.proargmodes::text[]
FROM pg_catalog.pg_proc p
JOIN extension_funcs ef ON ef.oid = p.oid
WHERE pg_function_is_visible(p.oid)
-- simply order all columns to keep subsequent runs stable
ORDER BY 1, 2, 3, 4, 5;
`

const catalogTmpl = `
// Code generated by sqlc-pg-gen. DO NOT EDIT.

package {{.Pkg}}

import (
	"github.com/sharpvik/sqlc/internal/sql/ast"
	"github.com/sharpvik/sqlc/internal/sql/catalog"
)

var funcs{{.GenFnName}} = []*catalog.Function {
    {{- range .Procs}}
	{
		Name: "{{.Name}}",
		Args: []*catalog.Argument{
			{{range .Args}}{
			{{- if .Name}}
			Name: "{{.Name}}",
			{{- end}}
			{{- if .HasDefault}}
			HasDefault: true,
			{{- end}}
			Type: &ast.TypeName{Name: "{{.TypeName}}"},
			{{- if ne .Mode "i" }}
			Mode: {{ .GoMode }},
			{{- end}}
			},
			{{end}}
		},
		ReturnType: &ast.TypeName{Name: "{{.ReturnTypeName}}"},
	},
	{{- end}}
}

func {{.GenFnName}}() *catalog.Schema {
	s := &catalog.Schema{Name: "{{ .SchemaName }}"}
	s.Funcs = funcs{{.GenFnName}}
	{{- if .Relations }}
	s.Tables = []*catalog.Table {
	    {{- range .Relations }}
		{
			Rel: &ast.TableName{
				Catalog: "{{.Catalog}}",
				Schema: "{{.SchemaName}}",
				Name: "{{.Name}}",
			},
			Columns: []*catalog.Column{
				{{- range .Columns}}
				{
					Name: "{{.Name}}",
					Type: ast.TypeName{Name: "{{.Type}}"},
					{{- if .IsNotNull}}
					IsNotNull: true,
					{{- end}}
					{{- if .IsArray}}
					IsArray: true,
					{{- end}}
					{{- if .Length }}
					Length: toPointer({{ .Length }}),
					{{- end}}
				},
				{{- end}}
			},
		},
		{{- end}}
	}
	{{- end }}
	return s
}
`

const loaderFuncTmpl = `
// Code generated by sqlc-pg-gen. DO NOT EDIT.

package postgresql

import (
	"github.com/sharpvik/sqlc/internal/engine/postgresql/contrib"
	"github.com/sharpvik/sqlc/internal/sql/catalog"
)

func loadExtension(name string) *catalog.Schema {
	switch name {
	{{- range .}}
	case "{{.Name}}":
		return contrib.{{.Func}}()
	{{- end}}
	}
	return nil
}
`

type tmplCtx struct {
	Pkg        string
	GenFnName  string
	SchemaName string
	Procs      []Proc
	Relations  []Relation
}

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func clean(arg string) string {
	arg = strings.TrimSpace(arg)
	arg = strings.Replace(arg, "\"any\"", "any", -1)
	arg = strings.Replace(arg, "\"char\"", "char", -1)
	arg = strings.Replace(arg, "\"timestamp\"", "char", -1)
	return arg
}

// writeFormattedGo executes `tmpl` with `data` as its context to the file `destPath`
func writeFormattedGo(tmpl *template.Template, data any, destPath string) error {
	out := bytes.NewBuffer([]byte{})
	err := tmpl.Execute(out, data)
	if err != nil {
		return err
	}
	code, err := format.Source(out.Bytes())
	if err != nil {
		return err
	}

	err = os.WriteFile(destPath, code, 0644)
	if err != nil {
		return err
	}

	return nil
}

// preserveLegacyCatalogBehavior maintain previous ordering and filtering
// that was manually done to the generated file pg_catalog.go.
// Some of the test depend on this ordering - in particular, function lookups
// where there might be multiple matching functions (due to type overloads)
// Until sqlc supports "smarter" looking up of these functions,
// preserveLegacyCatalogBehavior ensures there are no accidental test breakages
func preserveLegacyCatalogBehavior(allProcs []Proc) []Proc {
	// Preserve the legacy sort order of the end-to-end tests
	sort.SliceStable(allProcs, func(i, j int) bool {
		fnA := allProcs[i]
		fnB := allProcs[j]

		if fnA.Name == "lower" && fnB.Name == "lower" && len(fnA.ArgTypes) == 1 && fnA.ArgTypes[0] == "text" {
			return true
		}

		if fnA.Name == "generate_series" && fnB.Name == "generate_series" && len(fnA.ArgTypes) == 2 && fnA.ArgTypes[0] == "numeric" {
			return true
		}

		return false
	})

	procs := make([]Proc, 0, len(allProcs))
	for _, p := range allProcs {
		// Skip generating pg_catalog.concat to preserve legacy behavior
		if p.Name == "concat" {
			continue
		}

		procs = append(procs, p)
	}

	return procs
}

func databaseURL() string {
	dburl := os.Getenv("DATABASE_URL")
	if dburl != "" {
		return dburl
	}
	pgUser := os.Getenv("PG_USER")
	pgHost := os.Getenv("PG_HOST")
	pgPort := os.Getenv("PG_PORT")
	pgPass := os.Getenv("PG_PASSWORD")
	pgDB := os.Getenv("PG_DATABASE")
	if pgUser == "" {
		pgUser = "postgres"
	}
	if pgPass == "" {
		pgPass = "mysecretpassword"
	}
	if pgPort == "" {
		pgPort = "5432"
	}
	if pgHost == "" {
		pgHost = "127.0.0.1"
	}
	if pgDB == "" {
		pgDB = "dinotest"
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", pgUser, pgPass, pgHost, pgPort, pgDB)
}

func run(ctx context.Context) error {
	flag.Parse()

	dir := flag.Arg(0)
	if dir == "" {
		dir = filepath.Join("internal", "engine", "postgresql")
	}

	tmpl, err := template.New("").Parse(catalogTmpl)
	if err != nil {
		return err
	}
	conn, err := pgx.Connect(ctx, databaseURL())
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	schemas := []schemaToLoad{
		{
			Name:      "pg_catalog",
			GenFnName: "genPGCatalog",
			DestPath:  filepath.Join(dir, "pg_catalog.go"),
		},
		{
			Name:      "information_schema",
			GenFnName: "genInformationSchema",
			DestPath:  filepath.Join(dir, "information_schema.go"),
		},
	}

	for _, schema := range schemas {
		procs, err := readProcs(ctx, conn, schema.Name)
		if err != nil {
			return err
		}

		if schema.Name == "pg_catalog" {
			procs = preserveLegacyCatalogBehavior(procs)
		}

		relations, err := readRelations(ctx, conn, schema.Name)
		if err != nil {
			return err
		}

		err = writeFormattedGo(tmpl, tmplCtx{
			Pkg:        "postgresql",
			SchemaName: schema.Name,
			GenFnName:  schema.GenFnName,
			Procs:      procs,
			Relations:  relations,
		}, schema.DestPath)

		if err != nil {
			return err
		}
	}

	loaded := []extensionPair{}
	for _, extension := range extensions {
		name := strings.Replace(extension, "-", "_", -1)

		var funcName string
		for _, part := range strings.Split(name, "_") {
			funcName += strings.Title(part)
		}

		if _, err := conn.Exec(ctx, fmt.Sprintf("CREATE EXTENSION IF NOT EXISTS %q", extension)); err != nil {
			return fmt.Errorf("error creating %s: %s", extension, err)
		}

		rows, err := conn.Query(ctx, extensionFuncs, extension)
		if err != nil {
			return err
		}
		procs, err := scanProcs(rows)
		if err != nil {
			return err
		}
		if len(procs) == 0 {
			log.Printf("no functions in %s, skipping", extension)
			continue
		}

		// Preserve the legacy sort order of the end-to-end tests
		sort.SliceStable(procs, func(i, j int) bool {
			fnA := procs[i]
			fnB := procs[j]

			if extension == "pgcrypto" {
				if fnA.Name == "digest" && fnB.Name == "digest" && len(fnA.ArgTypes) == 2 && fnA.ArgTypes[0] == "text" {
					return true
				}
			}

			return false
		})

		extensionPath := filepath.Join(dir, "contrib", name+".go")
		err = writeFormattedGo(tmpl, tmplCtx{
			Pkg:        "contrib",
			SchemaName: "pg_catalog",
			GenFnName:  funcName,
			Procs:      procs,
		}, extensionPath)
		if err != nil {
			return fmt.Errorf("error generating extension %s: %w", extension, err)
		}

		loaded = append(loaded, extensionPair{Name: extension, Func: funcName})
	}

	extensionTmpl, err := template.New("").Parse(loaderFuncTmpl)
	if err != nil {
		return err
	}

	extensionLoaderPath := filepath.Join(dir, "extension.go")
	err = writeFormattedGo(extensionTmpl, loaded, extensionLoaderPath)
	if err != nil {
		return err
	}

	return nil
}

type schemaToLoad struct {
	// name is the name of a schema to load
	Name string
	// DestPath is the desination for the generate file
	DestPath string
	// The name of the function to generate for loading this schema
	GenFnName string
}

type extensionPair struct {
	Name string
	Func string
}

// https://www.postgresql.org/docs/current/contrib.html
var extensions = []string{
	"adminpack",
	"amcheck",
	// "auth_delay",
	// "auto_explain",
	// "bloom",
	"btree_gin",
	"btree_gist",
	"citext",
	"cube",
	"dblink",
	// "dict_int",
	// "dict_xsyn",
	"earthdistance",
	"file_fdw",
	"fuzzystrmatch",
	"hstore",
	"intagg",
	"intarray",
	"isn",
	"lo",
	"ltree",
	"pageinspect",
	// "passwordcheck",
	"pg_buffercache",
	"pg_freespacemap",
	"pg_prewarm",
	"pg_stat_statements",
	"pg_trgm",
	"pg_visibility",
	"pgcrypto",
	"pgrowlocks",
	"pgstattuple",
	"postgres_fdw",
	"seg",
	// "sepgsql",
	// "spi",
	"sslinfo",
	"tablefunc",
	"tcn",
	// "test_decoding",
	// "tsm_system_rows",
	// "tsm_system_time",
	"unaccent",
	"uuid-ossp",
	"xml2",
}
