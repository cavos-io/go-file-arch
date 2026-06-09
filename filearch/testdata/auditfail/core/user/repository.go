package user

type IUserRepository interface {
	Find(id string) error
}
