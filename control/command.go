package control

type Command struct {
	Command string
	Vin     string
	Body    map[string]interface{}
}
