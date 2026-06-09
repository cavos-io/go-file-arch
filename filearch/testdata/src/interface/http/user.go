package http

type UserResponse struct { // want `\[dto-file-name\]: DTO/request/response structs must be placed in dto\.go, \*_dto\.go, request\.go, or response\.go\. file name "user\.go" does not satisfy required fileName condition; detected declaration kind: struct`
	ID string
}
