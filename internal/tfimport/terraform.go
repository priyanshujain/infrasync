package tfimport

import (
	"fmt"
)

type importer struct {
	outputPath string
}

func NewImporter(outputPath string) (TerraformImporter, error) {
	if outputPath == "" {
		return nil, fmt.Errorf("output path cannot be empty")
	}
	return &importer{
		outputPath: outputPath,
	}, nil
}
