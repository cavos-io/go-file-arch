package user

type IUserService interface {
	Find(id string) error
}
