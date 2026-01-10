package module

import "time"

// GoListPackage matches the JSON shape produced by commands like `go list -json`.
type GoListPackage struct {
	Dir         string        `json:"Dir"`
	ImportPath  string        `json:"ImportPath"`
	Name        string        `json:"Name"`
	Target      string        `json:"Target"`
	Root        string        `json:"Root"`
	Module      *GoModule     `json:"Module,omitempty"`
	Match       []string      `json:"Match,omitempty"`
	Incomplete  bool          `json:"Incomplete,omitempty"`
	Stale       bool          `json:"Stale,omitempty"`
	StaleReason string        `json:"StaleReason,omitempty"`
	GoFiles     []string      `json:"GoFiles,omitempty"`
	Imports     []string      `json:"Imports,omitempty"`
	Deps        []string      `json:"Deps,omitempty"`
	DepsErrors  []GoDepsError `json:"DepsErrors,omitempty"`
}

type GoModule struct {
	Path      string    `json:"Path"`
	Version   string    `json:"Version"`
	Time      time.Time `json:"Time"` // RFC3339 (e.g. "2025-11-20T02:18:13Z")
	Indirect  bool      `json:"Indirect,omitempty"`
	Dir       string    `json:"Dir,omitempty"`
	GoMod     string    `json:"GoMod,omitempty"`
	GoVersion string    `json:"GoVersion,omitempty"`
	Sum       string    `json:"Sum,omitempty"`
	GoModSum  string    `json:"GoModSum,omitempty"`
}

type GoDepsError struct {
	ImportStack []string `json:"ImportStack,omitempty"`
	Pos         string   `json:"Pos,omitempty"`
	Err         string   `json:"Err,omitempty"`
}
