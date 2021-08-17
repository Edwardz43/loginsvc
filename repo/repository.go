package repo

type LoginRepository interface {
	Name(n string) (string, error)
}
