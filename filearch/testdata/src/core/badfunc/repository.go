package badfunc

func NewRepository() string { // want `\[core-repository-interface-only\]: core repository files may only define interfaces\. Move implementations to adapter/\*\*\. detected declaration kind: func`
	return "repository"
}
