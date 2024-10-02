package golang

import "github.com/sharpvik/sqlc/internal/codegen/golang/opts"

func parseDriver(sqlPackage string) opts.SQLDriver {
	switch sqlPackage {
	case opts.SQLPackagePGXV4:
		return opts.SQLDriverPGXV4
	case opts.SQLPackagePGXV5:
		return opts.SQLDriverPGXV5
	default:
		return opts.SQLDriverLibPQ
	}
}
