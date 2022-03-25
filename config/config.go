package config

var (
	closeFns []func()
)

//Option functions that init a configuration
type Option func()

//New create a singleton config struct
func New(fn ...Option) {
	for _, v := range fn {
		v()
	}
}

// Close close all connections
func Close() {
	for _, fn := range closeFns {
		fn()
	}
}
