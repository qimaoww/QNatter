package forward

type StartOptions struct {
	IP         string
	Port       int
	TargetIP   string
	TargetPort int
	UDP        bool
}

type None struct{}

func (None) Start(StartOptions) error {
	return nil
}

func (None) Stop() error {
	return nil
}
