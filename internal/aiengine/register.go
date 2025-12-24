package aiengine

var (
	m = make(map[string]AIEngineCreator)
)

type AIEngineCreator func(args interface{}) (IAIEngine, error)

func Create(name string, args interface{}) (IAIEngine, error) {
	if creator, ok := m[name]; ok {
		return creator(args)
	}
	return nil, nil
}
func Register(name string, creator AIEngineCreator) {
	if _, ok := m[name]; ok {
		panic("ai engine already registered")
	}
	m[name] = creator
}
