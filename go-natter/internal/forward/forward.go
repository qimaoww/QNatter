package forward

type StartOptions struct {
	IP            string
	SNATIP        string
	Port          int
	TargetIP      string
	TargetPort    int
	UDP           bool
	Interface     string
	RouteIdentity string
}

type None struct{}

func (None) Start(StartOptions) error {
	return nil
}

func (None) Stop() error {
	return nil
}
