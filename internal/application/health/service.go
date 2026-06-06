package apphealth

type Service interface {
	Health() bool
	Ready() bool
}

type service struct{}

func NewService() Service {
	return &service{}
}

func (s *service) Health() bool { return true }
func (s *service) Ready() bool  { return true }
