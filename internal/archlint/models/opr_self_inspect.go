package models

import "github.com/cavos-io/go-file-arch/internal/archlint/models/common"

type (
	CmdSelfInspectIn struct {
		ProjectPath string
		ArchFile    string
	}

	CmdSelfInspectOut struct {
		ModuleName    string                        `json:"ModuleName"`
		RootDirectory string                        `json:"RootDirectory"`
		LinterVersion string                        `json:"LinterVersion"`
		Notices       []CmdSelfInspectOutAnnotation `json:"Notices"`
		Suggestions   []CmdSelfInspectOutAnnotation `json:"Suggestions"`
	}

	CmdSelfInspectOutAnnotation struct {
		Text      string           `json:"Text"`
		Reference common.Reference `json:"Reference"`
	}
)
