package container

import (
	"github.com/logrusorgru/aurora/v3"

	"github.com/cavos-io/go-file-arch/internal/archlint/services/render"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/render/printer"
	"github.com/cavos-io/go-file-arch/internal/archlint/view"
)

func (c *Container) provideColorPrinter() *printer.ColorPrinter {
	return printer.NewColorPrinter(
		c.provideAurora(),
	)
}

func (c *Container) provideAurora() aurora.Aurora {
	return aurora.NewAurora(
		c.flags.UseColors,
	)
}

func (c *Container) ProvideRenderer() *render.Renderer {
	return render.NewRenderer(
		c.provideColorPrinter(),
		c.provideReferenceRender(),
		c.flags.OutputType,
		c.flags.OutputJsonOneLine,
		view.Templates,
	)
}
