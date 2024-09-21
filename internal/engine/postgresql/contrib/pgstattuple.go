// Code generated by sqlc-pg-gen. DO NOT EDIT.

package contrib

import (
	"github.com/sharpvik/sqlc/internal/sql/ast"
	"github.com/sharpvik/sqlc/internal/sql/catalog"
)

var funcsPgstattuple = []*catalog.Function{
	{
		Name: "pg_relpages",
		Args: []*catalog.Argument{
			{
				Name: "relname",
				Type: &ast.TypeName{Name: "regclass"},
			},
		},
		ReturnType: &ast.TypeName{Name: "bigint"},
	},
	{
		Name: "pg_relpages",
		Args: []*catalog.Argument{
			{
				Name: "relname",
				Type: &ast.TypeName{Name: "text"},
			},
		},
		ReturnType: &ast.TypeName{Name: "bigint"},
	},
	{
		Name: "pgstatginindex",
		Args: []*catalog.Argument{
			{
				Name: "relname",
				Type: &ast.TypeName{Name: "regclass"},
			},
		},
		ReturnType: &ast.TypeName{Name: "record"},
	},
	{
		Name: "pgstathashindex",
		Args: []*catalog.Argument{
			{
				Name: "relname",
				Type: &ast.TypeName{Name: "regclass"},
			},
		},
		ReturnType: &ast.TypeName{Name: "record"},
	},
	{
		Name: "pgstatindex",
		Args: []*catalog.Argument{
			{
				Name: "relname",
				Type: &ast.TypeName{Name: "regclass"},
			},
		},
		ReturnType: &ast.TypeName{Name: "record"},
	},
	{
		Name: "pgstatindex",
		Args: []*catalog.Argument{
			{
				Name: "relname",
				Type: &ast.TypeName{Name: "text"},
			},
		},
		ReturnType: &ast.TypeName{Name: "record"},
	},
	{
		Name: "pgstattuple",
		Args: []*catalog.Argument{
			{
				Name: "reloid",
				Type: &ast.TypeName{Name: "regclass"},
			},
		},
		ReturnType: &ast.TypeName{Name: "record"},
	},
	{
		Name: "pgstattuple",
		Args: []*catalog.Argument{
			{
				Name: "relname",
				Type: &ast.TypeName{Name: "text"},
			},
		},
		ReturnType: &ast.TypeName{Name: "record"},
	},
	{
		Name: "pgstattuple_approx",
		Args: []*catalog.Argument{
			{
				Name: "reloid",
				Type: &ast.TypeName{Name: "regclass"},
			},
		},
		ReturnType: &ast.TypeName{Name: "record"},
	},
}

func Pgstattuple() *catalog.Schema {
	s := &catalog.Schema{Name: "pg_catalog"}
	s.Funcs = funcsPgstattuple
	return s
}
