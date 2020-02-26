package pkg

type app struct {
	Name        string
	Description string
}

// App stores internal app state
var App = app{
	Name:        "pr-deployment-controller",
	Description: "Kubernetes operator for management of PR test environments in a cluster.",
}
