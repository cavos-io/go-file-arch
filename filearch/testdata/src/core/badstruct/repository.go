package badstruct

type RepositoryData struct { // want `\[core-repository-interface-only\]: core repository files may only define interfaces\. Move implementations to adapter/\*\*\. detected declaration kind: struct` `\[core-no-struct-outside-model\]: Structs in core should live under core/\*\*/model/\*\*\. detected declaration kind: struct`
	ID string
}
