package user

type Service struct { // want `\[core-no-struct-outside-model\]: Structs in core should live under core/\*\*/model/\*\*\. detected declaration kind: struct`
	Repository Repository
}
