package example

type Feature struct{}

type Client interface {
	UseFeature(Feature)
}
