package container

import (
	"github.com/cavos-io/go-file-arch/internal/archlint/services/checker"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/common/path"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/common/yaml/reference"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/project/holder"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/project/info"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/project/resolver"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/project/scanner"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/render/code"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/schema"
	specassembler "github.com/cavos-io/go-file-arch/internal/archlint/services/spec/assembler"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/spec/decoder"
	specvalidator "github.com/cavos-io/go-file-arch/internal/archlint/services/spec/validator"
)

func (c *Container) provideSpecAssembler() *specassembler.Assembler {
	return specassembler.NewAssembler(
		c.provideYamlSpecProvider(),
		c.provideSpecValidator(),
		c.providePathResolver(),
	)
}

func (c *Container) provideSpecValidator() *specvalidator.Validator {
	return specvalidator.NewValidator(
		c.providePathResolver(),
	)
}

func (c *Container) provideYamlSpecProvider() *decoder.Decoder {
	return decoder.NewDecoder(
		c.provideSourceCodeReferenceResolver(),
		c.provideJsonSchemaProvider(),
	)
}

func (c *Container) providePathResolver() *path.Resolver {
	return path.NewResolver()
}

func (c *Container) provideSourceCodeReferenceResolver() *reference.Resolver {
	return reference.NewResolver()
}

func (c *Container) provideReferenceRender() *code.Render {
	return code.NewRender(
		c.provideColorPrinter(),
	)
}

func (c *Container) provideSpecChecker() *checker.CompositeChecker {
	return checker.NewCompositeChecker(
		c.provideSpecImportsChecker(),
		c.provideSpecDeepScanChecker(),
	)
}

func (c *Container) provideSpecImportsChecker() *checker.Imports {
	return checker.NewImport(
		c.provideProjectFilesResolver(),
	)
}

func (c *Container) provideSpecDeepScanChecker() *checker.DeepScan {
	return checker.NewDeepScan(
		c.provideProjectFilesResolver(),
		c.provideReferenceRender(),
	)
}

func (c *Container) provideProjectFilesResolver() *resolver.Resolver {
	return resolver.NewResolver(
		c.provideProjectFilesScanner(),
		c.provideProjectFilesHolder(),
	)
}

func (c *Container) provideProjectFilesScanner() *scanner.Scanner {
	return scanner.NewScanner()
}

func (c *Container) provideProjectFilesHolder() *holder.Holder {
	return holder.NewHolder()
}

func (c *Container) provideProjectInfoAssembler() *info.Assembler {
	return info.NewAssembler()
}

func (c *Container) provideJsonSchemaProvider() *schema.Provider {
	return schema.NewProvider()
}
