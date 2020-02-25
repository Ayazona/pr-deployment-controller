package restore

// Restore defines the inferface used to restore databases
type Restore interface {
	Start() error
	Stop()
}
