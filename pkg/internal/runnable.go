package internal

// RunFunc defines a blocking function that can be called to start a service
type RunFunc func() error

// StopFunc defines a function that can be used to stop a function
type StopFunc func(error)

// Service defines a method that can be called to get the reunnable functions
type Service interface {
	Runnable() (RunFunc, StopFunc)
}
